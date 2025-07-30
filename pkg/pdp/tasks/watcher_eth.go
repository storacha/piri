package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"

	"github.com/cenkalti/backoff/v5"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	types2 "github.com/filecoin-project/lotus/chain/types"

	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/service/models"
)

// TODO allow this to be tuned based on network and user preferences for risk.
// original value from curio is 6, but a lower value is nice when testing againts calibration network

// MinConfidence defines how many blocks must be applied before we accept the message as applied.
// Synonymous with finality
const MinConfidence = 2

// Retry and concurrency configuration
const (
	// Maximum number of concurrent transaction checks
	defaultMaxConcurrentChecks = 10

	// Retry configuration
	defaultMaxAPIRetries = 10
)

type MessageWatcherEthClient interface {
	TransactionByHash(ctx context.Context, hash common.Hash) (tx *types.Transaction, isPending bool, err error)
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
}

// TransactionResult holds all the data needed to update a transaction in the database
type TransactionResult struct {
	TxHash               string
	Receipt              *types.Receipt
	Transaction          *types.Transaction
	ConfirmedBlockNumber int64
	TxDataJSON           []byte
	ReceiptJSON          []byte
	TxSuccess            bool
}

type MessageWatcherEth struct {
	db  *gorm.DB
	api MessageWatcherEthClient

	stopping, stopped chan struct{}

	updateCh        chan struct{}
	bestBlockNumber atomic.Pointer[big.Int]

	maxEthAPIRetries uint
}

// WatcherOption is a functional option for configuring MessageWatcherEth
type WatcherOption func(*MessageWatcherEth)

// WithMaxEthAPIRetries sets the maximum number of retries for the eth api
func WithMaxEthAPIRetries(n uint) WatcherOption {
	return func(mw *MessageWatcherEth) {
		mw.maxEthAPIRetries = n
	}
}

func NewMessageWatcherEth(db *gorm.DB, pcs *chainsched.Scheduler, api MessageWatcherEthClient, opts ...WatcherOption) (*MessageWatcherEth, error) {
	mw := &MessageWatcherEth{
		db:               db,
		api:              api,
		stopping:         make(chan struct{}),
		stopped:          make(chan struct{}),
		updateCh:         make(chan struct{}, 1),
		maxEthAPIRetries: defaultMaxAPIRetries,
	}

	// Apply options
	for _, opt := range opts {
		opt(mw)
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
	if bestBlockNumber == nil {
		log.Warn("best block number not yet available")
		return
	}

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

	if len(txs) == 0 {
		return
	}

	log.Debugw("processing pending transactions", "count", len(txs))

	// Create a channel for results
	resultCh := make(chan *TransactionResult, len(txs))

	// Use errgroup for better error handling and concurrency control
	g, ctx := errgroup.WithContext(ctx)
	// limit the number of concurrent executions (a semaphore)
	g.SetLimit(defaultMaxConcurrentChecks)

	// Process transactions concurrently
	for _, tx := range txs {
		txHash := common.HexToHash(tx.SignedTxHash)

		g.Go(func() error {
			result, err := mw.checkTransaction(ctx, txHash, bestBlockNumber)
			if err != nil {
				log.Errorf("failed to check transaction %s: %+v", txHash.Hex(), err)
				// Don't return error - continue processing other transactions
				return nil
			}

			if result != nil {
				select {
				case resultCh <- result:
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			return nil
		})
	}

	// Process results sequentially to avoid database conflicts
	var updateCount int
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for result := range resultCh {
			err := mw.updateTransaction(result)
			if err != nil {
				log.Errorf("failed to update transaction %s: %+v", result.TxHash, err)
				continue
			}
			updateCount++
		}
	}()

	// Wait for all goroutines to complete
	if err := g.Wait(); err != nil {
		log.Errorf("error in transaction processing: %+v", err)
	}
	// close the results channel, terminating the above go routine.
	close(resultCh)
	// wait for the go routine to complete updaing transactions from results channel
	wg.Wait()

	log.Debugw("completed transaction updates", "updated", updateCount, "total", len(txs))
}

// checkTransaction fetches transaction data with retry logic
func (mw *MessageWatcherEth) checkTransaction(ctx context.Context, txHash common.Hash, bestBlockNumber *big.Int) (*TransactionResult, error) {
	// First, get the receipt with retries
	receipt, err := mw.getReceiptWithRetry(ctx, txHash)
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			// Transaction is still pending
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get receipt after retries: %w", err)
	}

	// Check if the transaction has enough confirmations
	confirmations := new(big.Int).Sub(bestBlockNumber, receipt.BlockNumber)
	if confirmations.Cmp(big.NewInt(MinConfidence)) < 0 {
		// Not enough confirmations yet
		return nil, nil
	}

	// Get the transaction data with retries
	txData, err := mw.getTransactionWithRetry(ctx, txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction data after retries: %w", err)
	}

	// Marshal the data
	txDataJSON, err := json.Marshal(txData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transaction data: %w", err)
	}

	receiptJSON, err := json.Marshal(receipt)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal receipt data: %w", err)
	}

	return &TransactionResult{
		TxHash:               txHash.Hex(),
		Receipt:              receipt,
		Transaction:          txData,
		ConfirmedBlockNumber: receipt.BlockNumber.Int64(),
		TxDataJSON:           txDataJSON,
		ReceiptJSON:          receiptJSON,
		TxSuccess:            receipt.Status == 1,
	}, nil
}

// getReceiptWithRetry fetches a transaction receipt with exponential backoff retry
func (mw *MessageWatcherEth) getReceiptWithRetry(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	return backoff.Retry(ctx, func() (*types.Receipt, error) {
		r, err := mw.api.TransactionReceipt(ctx, txHash)
		// Don't retry on NotFound errors
		if errors.Is(err, ethereum.NotFound) {
			return nil, backoff.Permanent(err)
		}
		return r, err
	}, backoff.WithMaxTries(mw.maxEthAPIRetries), backoff.WithBackOff(backoff.NewExponentialBackOff()))
}

// getTransactionWithRetry fetches transaction data with exponential backoff retry
func (mw *MessageWatcherEth) getTransactionWithRetry(ctx context.Context, txHash common.Hash) (*types.Transaction, error) {
	return backoff.Retry(ctx, func() (*types.Transaction, error) {
		t, _, err := mw.api.TransactionByHash(ctx, txHash)
		// Don't retry on NotFound errors
		if errors.Is(err, ethereum.NotFound) {
			return nil, backoff.Permanent(err)
		}
		return t, err
	}, backoff.WithMaxTries(mw.maxEthAPIRetries), backoff.WithBackOff(backoff.NewExponentialBackOff()))
}

// updateTransaction updates a single transaction in the database
func (mw *MessageWatcherEth) updateTransaction(result *TransactionResult) error {
	return mw.db.Model(&models.MessageWaitsEth{}).
		Where("signed_tx_hash = ?", result.TxHash).
		Updates(models.MessageWaitsEth{
			WaiterMachineID:      nil,
			ConfirmedBlockNumber: models.Ptr(result.ConfirmedBlockNumber),
			ConfirmedTxHash:      result.Receipt.TxHash.Hex(),
			ConfirmedTxData:      result.TxDataJSON,
			TxStatus:             "confirmed",
			TxReceipt:            result.ReceiptJSON,
			TxSuccess:            &result.TxSuccess,
		}).Error
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
