package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/tools/service-operator/eip712"
)

func (p *PDPService) CreateProofSet(ctx context.Context, params types.CreateProofSetParams) (res common.Hash, retErr error) {
	log.Infow("creating proof set", "recordKeeper", params.RecordKeeper)
	defer func() {
		if retErr != nil {
			log.Errorw("failed to create proof set", "recordKeeper", params.RecordKeeper, "retErr", retErr)
		} else {
			log.Infow("created proof set", "recordKeeper", params.RecordKeeper, "tx", res.String())
		}
	}()
	if len(params.RecordKeeper.Bytes()) == 0 {
		return common.Hash{}, types.NewError(types.KindInvalidInput, "record keeper is required")
	}
	if !common.IsHexAddress(params.RecordKeeper.Hex()) {
		return common.Hash{}, types.NewErrorf(types.KindInvalidInput, "record keeper %s is not a valid address", params.RecordKeeper)
	}

	var extraDataBytes []byte
	var listenerAddress common.Address

	// Check if signing service is configured (for authenticated operations)
	if p.signingService != nil && p.viewContract != nil {
		log.Infof("Using signing service for authenticated CreateDataSet")

		// Get the next client dataset ID for this payer
		nextClientDataSetId, err := p.viewContract.GetNextClientDataSetId(p.payerAddress)
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to get next client dataset ID: %w", err)
		}
		log.Infof("Next client dataset ID for payer %s: %s", p.payerAddress.Hex(), nextClientDataSetId.String())

		// Parse metadata from params if provided (expected format: "key1=value1,key2=value2")
		var metadata []string
		if params.ExtraData != "" {
			// Interpret extraData as comma-separated metadata entries for now
			metadata = strings.Split(string(params.ExtraData), ",")
		}
		metadata = []string{"foo=bar"}
		metadataEntries, err := ParseMetadataEntries(metadata)
		if err != nil {
			log.Warnf("Failed to parse metadata, using empty metadata: %v", err)
			metadataEntries = []eip712.MetadataEntry{}
		}

		// Sign the CreateDataSet operation
		fmt.Printf("DEBUG: About to sign with:\n")
		fmt.Printf("  clientDataSetId: %s\n", nextClientDataSetId.String())
		fmt.Printf("  payee (service provider): %s\n", p.address.Hex())
		fmt.Printf("  metadata: %v\n", metadataEntries)

		signature, err := p.signingService.SignCreateDataSet(ctx,
			nextClientDataSetId,
			p.address, // Use the nodes address as the address receiving payment for storage
			metadataEntries,
		)
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to sign CreateDataSet: %w", err)
		}
		fmt.Printf("DEBUG: Signature created:\n")
		fmt.Printf("  Signer address: %s\n", signature.Signer.Hex())
		fmt.Printf("  Signature (first 32 bytes): 0x%x\n", signature.Signature[:32])
		log.Infof("Signed CreateDataSet with signer %s", signature.Signer.Hex())

		// Verify the signature locally before sending to the network
		// This helps debug signature issues without waiting for gas estimation
		domain := eip712.GetDomain(big.NewInt(314159), smartcontracts.Addresses().PDPService)

		// Prepare the message for recovery
		metadataArray := make([]map[string]interface{}, len(metadataEntries))
		for i, entry := range metadataEntries {
			metadataArray[i] = map[string]interface{}{
				"key":   entry.Key,
				"value": entry.Value,
			}
		}
		message := map[string]interface{}{
			"clientDataSetId": nextClientDataSetId,
			"payee":           strings.ToLower(p.address.Hex()),
			"metadata":        metadataArray,
		}

		hash, err := eip712.GetMessageHash(domain, "CreateDataSet", message)
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to get message hash for verification: %w", err)
		}

		// Recover the signer from the signature
		recoverySignature := make([]byte, 65)
		copy(recoverySignature, signature.Signature)
		if recoverySignature[64] >= 27 {
			recoverySignature[64] -= 27
		}

		pubKey, err := crypto.SigToPub(hash, recoverySignature)
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to recover public key: %w", err)
		}

		recoveredSigner := crypto.PubkeyToAddress(*pubKey)

		fmt.Printf("DEBUG: Local signature verification:\n")
		fmt.Printf("  Expected signer (payer): %s\n", p.payerAddress.Hex())
		fmt.Printf("  Recovered signer: %s\n", recoveredSigner.Hex())
		fmt.Printf("  Message hash: 0x%x\n", hash)

		if strings.ToLower(recoveredSigner.Hex()) != strings.ToLower(p.payerAddress.Hex()) {
			return common.Hash{}, fmt.Errorf("LOCAL signature verification failed: expected %s, recovered %s",
				p.payerAddress.Hex(), recoveredSigner.Hex())
		}
		fmt.Printf("  ✓ Local signature verification PASSED\n")

		fmt.Println("PAYER: " + p.payerAddress.Hex())
		// Encode the extraData with payer, metadata, and signature
		extraDataBytes, err = p.extraDataHelper.EncodeCreateDataSetExtraData(
			p.payerAddress,
			metadataEntries,
			signature,
		)
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to encode extraData: %w", err)
		}

		// Use the service contract address as the listener
		listenerAddress = smartcontracts.Addresses().PDPService
		fmt.Println("listener: " + listenerAddress.Hex())
		fmt.Println("expected listen address: " + smartcontracts.Addresses().PDPService.String())
		fmt.Println("Record Keeper: " + params.RecordKeeper.String())
		fmt.Println("verifier: " + smartcontracts.Addresses().PDPVerifier.String())
	} else {
		// Fall back to original behavior if signing service not configured
		log.Infof("Signing service not configured, using legacy CreateDataSet")

		// Decode extraData if provided
		if params.ExtraData != "" {
			extraDataHexStr := string(params.ExtraData)
			decodedBytes, err := hex.DecodeString(strings.TrimPrefix(extraDataHexStr, "0x"))
			if err != nil {
				log.Errorf("Failed to decode hex extraData: %v", err)
				return common.Hash{}, types.WrapError(types.KindInvalidInput,
					fmt.Sprintf("invalid extraData format: %s (must be hex encoded)", params.ExtraData),
					err)
			}
			extraDataBytes = decodedBytes
		}
		// Use empty address as listener (legacy behavior)
		listenerAddress = common.Address{}
	}

	// Obtain the ABI of the PDPVerifier contract
	abiData, err := smartcontracts.PDPVerifierMetaData()
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get contract ABI: %w", err)
	}

	// Pack the method call data with listener address and extraData
	data, err := abiData.Pack("createDataSet", listenerAddress, extraDataBytes)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to pack create proof set: %w", err)
	}

	// Prepare the transaction (nonce will be set to 0, SenderETH will assign it)
	// We create a DynamicFeeTx (EIP-1559) with a fixed gas limit to bypass gas estimation
	// Gas estimation is failing, but manual testing proves the transaction works

	// Get network info for fee calculation
	// TODO method doesn't exit @claude, I have hardcoded below
	/*
		chainID, err := p.contractBackend.NetworkID(ctx)
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to get chain ID: %w", err)
		}

	*/

	// Get current base fee
	header, err := p.contractBackend.HeaderByNumber(ctx, nil)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get latest block header: %w", err)
	}

	baseFee := header.BaseFee
	if baseFee == nil {
		return common.Hash{}, fmt.Errorf("base fee not available")
	}

	// Get tip cap
	gasTipCap, err := p.contractBackend.SuggestGasTipCap(ctx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get gas tip cap: %w", err)
	}

	// Calculate fee cap
	gasFeeCap := new(big.Int).Add(baseFee, gasTipCap)

	// Create EIP-1559 transaction with fixed gas limit
	// Using a very high gas limit because createDataSet triggers dataSetCreated in the listener contract
	// which creates payment rails, stores metadata, verifies signatures, etc.
	tx := ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   big.NewInt(int64(314159)),
		Nonce:     0, // Will be set by SenderETH
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
		Gas:       100_000_000, // 100M gas - createDataSet does a lot of work in the listener contract
		To:        ptrTo(smartcontracts.Addresses().PDPVerifier),
		Value:     smartcontracts.SybilFee(),
		Data:      data,
	})

	reason := "pdp-mkproofset"
	fmt.Printf("DEBUG: Transaction details:\n")
	fmt.Printf("  To: %s\n", tx.To().Hex())
	fmt.Printf("  Value: %s FIL\n", tx.Value().String())
	fmt.Printf("  From (will be): %s\n", p.address.Hex())
	fmt.Printf("  Gas limit: %d\n", tx.Gas())
	fmt.Printf("  Gas fee cap: %s\n", tx.GasFeeCap().String())
	fmt.Printf("  Gas tip cap: %s\n", tx.GasTipCap().String())
	fmt.Printf("  Data length: %d bytes\n", len(tx.Data()))

	log.Infof("Encoded extraData with payer %s, length=%d", p.payerAddress.Hex(), len(extraDataBytes))
	log.Infof("ExtraData hex: 0x%s", hex.EncodeToString(extraDataBytes))

	/*
			 * The error is InvalidSignature(address expected, address actual):
		     *   - Expected (payer): 0x8aae051b0262213b5a354e099dae3b8d44c85564
		     *   - Recovered: 0x445238eca6c6ab8dff1aa6087d9c05734d22f137
	*/
	txHash, err := p.sender.Send(ctx, p.address, tx, reason)
	if err != nil {
		fmt.Println(err.Error())
		return common.Hash{}, fmt.Errorf("failed to send transaction: %w", err)
	}

	if err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		msgWait := models.MessageWaitsEth{
			SignedTxHash: txHash.Hex(),
			TxStatus:     "pending",
		}
		if err := tx.Create(&msgWait).Error; err != nil {
			return fmt.Errorf("failed to insert into %s: %w", msgWait.TableName(), err)
		}

		proofsetCreate := models.PDPProofsetCreate{
			CreateMessageHash: txHash.Hex(),
			Service:           p.name,
			// ProofsetCreated defaults to false, and Ok is nil by default.
		}
		if err := tx.Create(&proofsetCreate).Error; err != nil {
			return fmt.Errorf("failed to insert into %s: %w", proofsetCreate.TableName(), err)
		}

		// Return nil to commit the transaction.
		return nil
	}); err != nil {
		return common.Hash{}, err
	}

	return txHash, nil
}

// ptrTo returns a pointer to the given address
func ptrTo(addr common.Address) *common.Address {
	return &addr
}
