package service

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/contract"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/types"
)

func (p *PDPService) CreateProofSet(ctx context.Context, recordKeeper common.Address) (res common.Hash, retErr error) {
	log.Infow("creating proof set", "recordKeeper", recordKeeper)
	defer func() {
		if retErr != nil {
			log.Errorw("failed to create proof set", "recordKeeper", recordKeeper, "retErr", retErr)
		} else {
			log.Infow("created proof set", "recordKeeper", recordKeeper, "tx", res.String())
		}
	}()
	if len(recordKeeper.Bytes()) == 0 {
		return common.Hash{}, types.NewError(types.KindInvalidInput, "record keeper is required")
	}
	if !common.IsHexAddress(recordKeeper.Hex()) {
		return common.Hash{}, types.NewErrorf(types.KindInvalidInput, "record keeper %s is not a valid address", recordKeeper)
	}

	// Obtain the ABI of the PDPVerifier contract
	abiData, err := contract.PDPVerifierMetaData()
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get contract ABI: %w", err)
	}

	// Pack the method call data
	data, err := abiData.Pack("createProofSet", recordKeeper, []byte{})
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to pack create proof set: %w", err)
	}

	// Prepare the transaction (nonce will be set to 0, SenderETH will assign it)
	tx := ethtypes.NewTransaction(
		0,
		contract.Addresses().PDPVerifier,
		contract.SybilFee(),
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
