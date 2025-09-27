package service

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/eip712"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
	"github.com/storacha/piri/pkg/pdp/types"
)

func (p *PDPService) CreateProofSet(ctx context.Context, params types.CreateProofSetParams) (res common.Hash, retErr error) {
	if _, err := p.RegisterProvider(ctx, RegisterProviderParams{
		Name:        "Testing1",
		Description: "Testing1",
	}); err != nil {
		return common.Hash{}, err
	}
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

	// TODO this might not be the record keeper as the verifing addres, may instead be the service contract
	// TODO: Need to track clientDataSetId properly - for now using 0 for first dataset
	clientDataSetId := big.NewInt(1)
	signer, err := p.wallet.NewContractMessageSigner(ctx, p.address, params.RecordKeeper)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to create signer: %w", err)
	}
	extraDataBytes, err := eip712.CreateDataSetExtraData(signer, clientDataSetId, p.address, []eip712.MetadataEntry{
		{
			Key:   "foo",
			Value: "bar",
		},
	})
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to create extra data: %w", err)
	}
	/*
		// Decode extraData if provided
		var extraDataBytes []byte
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
	*/

	// Obtain the ABI of the PDPVerifier contract
	abiData, err := smartcontracts.PDPVerifierMetaData()
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get contract ABI: %w", err)
	}

	// REVIEW: We cannot use a record keeper right now since the only supported one is the
	// Filecoin Warm Storage Service contract. And membership to that contract is gated
	// but the owner, which is not storacha.
	// This code can function without a record keeper, but will require some hacks
	// Pack the method call data
	data, err := abiData.Pack("createDataSet",
		common.HexToAddress(smartcontracts.PDPFilecoinWarmStorageServiceRecordKeeperAddress), extraDataBytes)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to pack create proof set: %w", err)
	}

	// Prepare the transaction (nonce will be set to 0, SenderETH will assign it)
	tx := ethtypes.NewTransaction(
		0,
		smartcontracts.Addresses().PDPVerifier,
		smartcontracts.SybilFee(),
		0,
		nil,
		data,
	)

	reason := "pdp-mkproofset"
	txHash, err := p.sender.Send(ctx, p.address, tx, reason)
	if err != nil {
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
