package service

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/contract"
	"github.com/storacha/piri/pkg/pdp/service/models"
)

func (p *PDPService) DeleteProofSet(ctx context.Context, proofSetID uint64) (res common.Hash, retErr error) {
	log.Infow("deleting proof set", "proofSetID", proofSetID)
	defer func() {
		if retErr != nil {
			log.Errorw("failed to delete proof set", "proofSetID", proofSetID, "err", retErr)
		} else {
			log.Infow("deleted proof set", "proofSetID", proofSetID, "response", res)
		}
	}()

	// Get the ABI and pack the transaction data
	abiData, err := contract.PDPVerifierMetaData()
	if err != nil {
		return common.Hash{}, fmt.Errorf("get contract ABI: %w", err)
	}

	// Pack the method call data for deleteProofSet
	data, err := abiData.Pack("deleteProofSet",
		big.NewInt(int64(proofSetID)),
		[]byte{}, // extraData parameter
	)
	if err != nil {
		return common.Hash{}, fmt.Errorf("pack ABI method call: %w", err)
	}

	// Prepare the transaction
	ethTx := types.NewTransaction(
		0, // nonce will be set by SenderETH
		contract.Addresses().PDPVerifier,
		big.NewInt(0), // value
		0,             // gas limit (will be estimated)
		nil,           // gas price (will be set by SenderETH)
		data,
	)

	// Send the transaction
	reason := "pdp-delete-proofset"
	txHash, err := p.sender.Send(ctx, p.address, ethTx, reason)
	if err != nil {
		return common.Hash{}, fmt.Errorf("send transaction: %w", err)
	}

	// Schedule deletion tracking using a transaction
	if err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Insert into message_waits_eth
		m := models.MessageWaitsEth{
			SignedTxHash: txHash.String(),
			TxStatus:     "pending",
		}
		tx.WithContext(ctx).Create(&m)
		return nil
	}); err != nil {
		return common.Hash{}, fmt.Errorf("scheduling delete proof set %d: %w", proofSetID, err)
	}

	return txHash, nil
}
