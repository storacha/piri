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

func (p *PDPService) RemoveRoot(ctx context.Context, proofSetID uint64, rootID uint64) (common.Hash, error) {
	// Get the ABI and pack the transaction data
	abiData, err := contract.PDPVerifierMetaData()
	if err != nil {
		return common.Hash{}, fmt.Errorf("get contract ABI: %w", err)
	}

	// TODO should probably check if we even have the proof set before scheduling a removal

	// Pack the method call data
	data, err := abiData.Pack("scheduleRemovals",
		big.NewInt(int64(proofSetID)),
		[]*big.Int{big.NewInt(int64(rootID))},
		[]byte{},
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
	reason := "pdp-delete-root"
	txHash, err := p.sender.Send(ctx, p.address, ethTx, reason)
	if err != nil {
		return common.Hash{}, fmt.Errorf("send transaction: %w", err)
	}

	// Schedule deletion of the root from the proof set using a transaction
	if err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Insert into message_waits_eth
		m := models.MessageWaitsEth{
			SignedTxHash: txHash.String(),
			TxStatus:     "pending",
		}
		tx.WithContext(ctx).Create(&m)
		return nil
	}); err != nil {
		return common.Hash{}, fmt.Errorf("shceduling delete root %d from proofset %d: %w", rootID, proofSetID, err)
	}

	return txHash, nil
}
