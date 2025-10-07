package service

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
)

func (p *PDPService) RemoveRoot(ctx context.Context, proofSetID uint64, rootID uint64) (res common.Hash, retErr error) {
	log.Infow("removing root", "proofSetID", proofSetID, "rootID", rootID)
	defer func() {
		if retErr != nil {
			log.Errorw("failed to remove root", "proofSetID", proofSetID, "rootID", rootID, "err", retErr)
		} else {
			log.Infow("removed root", "proofSetID", proofSetID, "rootID", rootID, "response", res)
		}
	}()
	// Get the ABI and pack the transaction data
	abiData, err := smartcontracts.PDPVerifierMetaData()
	if err != nil {
		return common.Hash{}, fmt.Errorf("get contract ABI: %w", err)
	}

	// TODO should probably check if we even have the proof set before scheduling a removal

	// TODO this will surely fail without extraData as a signature.
	// Pack the method call data
	data, err := abiData.Pack("schedulePieceDeletions",
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
		smartcontracts.Addresses().PDPVerifier,
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
