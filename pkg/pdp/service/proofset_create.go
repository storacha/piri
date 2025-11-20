package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"

	"github.com/storacha/filecoin-services/go/eip712"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
)

func (p *PDPService) CreateProofSet(ctx context.Context) (res common.Hash, retErr error) {
	log.Infow("creating proof set")
	defer func() {
		if retErr != nil {
			log.Errorw("failed to create proof set", "error", retErr)
		} else {
			log.Infow("created proof set", "tx", res.String())
		}
	}()

	// Check if the provider is both registered and approved
	if err := p.RequireProviderApproved(ctx); err != nil {
		return common.Hash{}, err
	}

	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		return common.Hash{}, fmt.Errorf("failed to generate nonce: %w", err)
	}
	nonce := new(big.Int).SetBytes(nonceBytes)

	var metadataEntries []eip712.MetadataEntry
	// request a signature for creating the dataset from the signing service
	signature, err := p.signingService.SignCreateDataSet(ctx,
		p.id,
		nonce,
		p.address, // Use the nodes address as the address receiving payment for storage
		metadataEntries,
	)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to sign CreateDataSet: %w", err)
	}

	// Encode the extraData with payer, metadata, and signature
	extraDataBytes, err := p.edc.EncodeCreateDataSetExtraData(
		p.cfg.PayerAddress,
		nonce,
		metadataEntries,
		signature,
	)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to encode extraData: %w", err)
	}

	// Obtain the ABI of the PDPVerifier contract
	abiData, err := p.verifierContract.GetABI()
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get contract ABI: %w", err)
	}

	// Pack the method call data with listener address and extraData
	data, err := abiData.Pack("createDataSet", p.cfg.Contracts.Service, extraDataBytes)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to pack create proof set: %w", err)
	}

	// Prepare the transaction (nonce will be set to 0, SenderETH will assign it)
	tx := ethtypes.NewTransaction(
		0,
		p.cfg.Contracts.Verifier,
		smartcontracts.SybilFee,
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
