package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"

	types2 "github.com/filecoin-project/lotus/chain/types"

	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service/models"
)

// TODO allow this to be tuned based on network and user preferences for risk.
// original value from curio is 6, but a lower value is nice when testing against calibration network

// MinConfidence defines how many blocks must be applied before we accept the message as applied.
// Synonymous with finality
const MinConfidence = 2

type MessageWatcherEthClient interface {
	TransactionByHash(ctx context.Context, hash common.Hash) (tx *types.Transaction, isPending bool, err error)
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
}

type MessageWatcherEth struct {
	db  *gorm.DB
	api MessageWatcherEthClient

	stopping, stopped chan struct{}

	updateCh        chan struct{}
	bestBlockNumber atomic.Pointer[big.Int]
}

func NewMessageWatcherEth(db *gorm.DB, pcs *scheduler.Chain, api MessageWatcherEthClient) (*MessageWatcherEth, error) {
	mw := &MessageWatcherEth{
		db:       db,
		api:      api,
		stopping: make(chan struct{}),
		stopped:  make(chan struct{}),
		updateCh: make(chan struct{}, 1),
	}
	go mw.run()
	if err := pcs.AddHandler(mw.processHeadChange); err != nil {
		return nil, err
	}
	return mw, nil
}

func (mw *MessageWatcherEth) run() {
	defer close(mw.stopped)

	for {
		select {
		case <-mw.stopping:
			// TODO: cleanup assignments
			return
		case <-mw.updateCh:
			mw.update()
		}
	}
}

func (mw *MessageWatcherEth) update() {
	ctx := context.Background()

	bestBlockNumber := mw.bestBlockNumber.Load()

	confirmedBlockNumber := new(big.Int).Sub(bestBlockNumber, big.NewInt(MinConfidence))
	if confirmedBlockNumber.Sign() < 0 {
		// Not enough blocks yet
		return
	}

	machineID := 1

	// Assign pending transactions with null owner to ourselves
	{
		res := mw.db.Model(&models.MessageWaitsEth{}).
			Where("waiter_machine_id IS NULL").
			Where("tx_status = ?", "pending").
			Update("waiter_machine_id", machineID)
		if res.Error != nil {
			log.Errorf("failed to assign pending transactions: %+v", res.Error)
			return
		}
		if res.RowsAffected > 0 {
			log.Debugw("assigned pending transactions to ourselves", "assigned", res.RowsAffected)
		}
	}

	// Get transactions assigned to us
	var txs []struct {
		SignedTxHash string
	}
	err := mw.db.Model(&models.MessageWaitsEth{}).
		Select("signed_tx_hash").
		Where("waiter_machine_id = ?", machineID).
		Where("tx_status = ?", "pending").
		Limit(10000).
		Scan(&txs).Error
	if err != nil {
		log.Errorf("failed to get assigned transactions: %+v", err)
		return
	}

	// Check if any of the transactions we have assigned are now confirmed
	for _, tx := range txs {
		txHash := common.HexToHash(tx.SignedTxHash)

		receipt, err := mw.api.TransactionReceipt(ctx, txHash)
		if err != nil {
			if errors.Is(err, ethereum.NotFound) {
				// Transaction is still pending
				continue
			}
			log.Errorf("failed to get transaction receipt for hash %s: %+v", txHash.Hex(), err)
			return
		}

		// Check if the transaction has enough confirmations
		confirmations := new(big.Int).Sub(bestBlockNumber, receipt.BlockNumber)
		if confirmations.Cmp(big.NewInt(MinConfidence)) < 0 {
			// Not enough confirmations yet
			continue
		}

		// Get the transaction data
		txData, _, err := mw.api.TransactionByHash(ctx, txHash)
		if err != nil {
			if errors.Is(err, ethereum.NotFound) {
				log.Errorf("transaction data not found for txHash: %s", txHash.Hex())
				continue
			}
			log.Errorf("failed to get transaction by hash %s: %+v", txHash.Hex(), err)
			return
		}

		txDataJSON, err := json.Marshal(txData)
		if err != nil {
			log.Errorf("failed to marshal transaction data for hash %s: %+v", txHash.Hex(), err)
			return
		}

		receiptJSON, err := json.Marshal(receipt)
		if err != nil {
			log.Errorf("failed to marshal receipt data for hash %s: %+v", txHash.Hex(), err)
			return
		}

		txStatus := "confirmed"
		txSuccess := receipt.Status == 1

		// Update the database
		err = mw.db.Model(&models.MessageWaitsEth{}).
			Where("signed_tx_hash = ?", tx.SignedTxHash).
			Updates(models.MessageWaitsEth{
				WaiterMachineID:      nil,
				ConfirmedBlockNumber: models.Ptr(receipt.BlockNumber.Int64()),
				ConfirmedTxHash:      receipt.TxHash.Hex(),
				ConfirmedTxData:      txDataJSON,
				TxStatus:             txStatus,
				TxReceipt:            receiptJSON,
				TxSuccess:            &txSuccess,
			}).Error
		if err != nil {
			log.Errorf("failed to update message wait for hash %s: %+v", txHash.Hex(), err)
			return
		}
	}
}

func (mw *MessageWatcherEth) Stop(ctx context.Context) error {
	close(mw.stopping)
	select {
	case <-mw.stopped:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (mw *MessageWatcherEth) processHeadChange(ctx context.Context, revert, apply *types2.TipSet) error {
	if apply != nil {
		mw.bestBlockNumber.Store(big.NewInt(int64(apply.Height())))
		select {
		case mw.updateCh <- struct{}{}:
		default:
		}
	}
	return nil
}
