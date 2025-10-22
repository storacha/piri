package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/models"
)

// TODO: this method treats the database as the source of truth for transaction confirmation
// watch_eth.go uses a `MinConfidence` field before considering a transaction confirmed
// we could improve this methods performance by querying the chain directly for confirmation, bypassing the
// database confidence logic, at the risk of missing chain forks.

// WaitForConfirmation blocks until the given transaction hash is confirmed on chain.
// This method polls the database for the transaction status and returns when the
// transaction moves from "pending" to "confirmed" status.
// Returns an error if the transaction fails or if polling encounters an error.
func (p *PDPService) WaitForConfirmation(ctx context.Context, txHash common.Hash, wait time.Duration) error {
	log.Infow("waiting for transaction confirmation", "txHash", txHash.Hex())
	transactionConfirmed := func() (interface{}, error) {
		var waits models.MessageWaitsEth
		err := p.db.WithContext(ctx).
			Where("signed_tx_hash = ?", txHash.Hex()).
			First(&waits).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// if the transaction doesn't exit in the database, then this represents a developer error or a system failure.
				return nil, backoff.Permanent(fmt.Errorf("transaction %s not found in message_waits_eth", txHash))
			}
			return nil, fmt.Errorf("failed to check transaction status: %w", err)
		}

		// Check if transaction is confirmed, confirmed implies the transaction landed on chain, but not its success.
		// this field with either be "pending" or "confirmed".
		if waits.TxStatus == "confirmed" {
			// safety check, once confirmed should always be set
			if waits.TxSuccess == nil {
				return nil, fmt.Errorf("transaction %s confirmed but success status is null", txHash.Hex())
			}

			if !*waits.TxSuccess {
				// Transaction failed on chain
				receiptInfo := ""
				if waits.TxReceipt != nil {
					receiptInfo = fmt.Sprintf(", receipt: %s", string(waits.TxReceipt))
				}

				return nil, backoff.Permanent(fmt.Errorf("transaction %s failed, receipt: %s", txHash.Hex(), receiptInfo))
			}

			log.Infow("transaction confirmed", "txHash", txHash.Hex(), "blockNumber", waits.ConfirmedBlockNumber)
			return nil, nil
		}
		// else pending
		return nil, fmt.Errorf("transaction %s not confirmed", txHash.Hex())
	}

	_, err := backoff.Retry(
		ctx,
		transactionConfirmed,
		backoff.WithBackOff(backoff.NewConstantBackOff(5*time.Second)),
		backoff.WithMaxElapsedTime(wait),
	)
	return err
}
