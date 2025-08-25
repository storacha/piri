package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"

	chaintypes "github.com/filecoin-project/lotus/chain/types"

	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/service/contract"
	"github.com/storacha/piri/pkg/pdp/service/models"
)

type ProofSetDelete struct {
	DeleteMessageHash string `db:"delete_message_hash"`
	Service           string `db:"service"`
}

func NewWatcherDeleteProofSet(
	db *gorm.DB,
	ethClient bind.ContractBackend,
	contractClient contract.PDP,
	pcs *chainsched.Scheduler,
) error {
	log.Infow("Initializing proof set deletion watcher")
	if err := pcs.AddHandler(func(ctx context.Context, revert, apply *chaintypes.TipSet) error {
		log.Debugw("Chain update triggered proof set deletion check", "tipset_height", apply.Height())
		err := processPendingProofSetDeletes(ctx, db, ethClient, contractClient)
		if err != nil {
			log.Warnw("Failed to process pending proof set deletes", "error", err, "tipset_height", apply.Height())
		}
		return nil
	}); err != nil {
		log.Errorw("Failed to register proof set deletion watcher handler", "error", err)
		return err
	}
	log.Infow("Successfully registered proof set deletion watcher")
	return nil
}

func processPendingProofSetDeletes(
	ctx context.Context,
	db *gorm.DB,
	ethClient bind.ContractBackend,
	contractClient contract.PDP,
) error {
	log.Debugw("Querying for pending proof set deletions", "query_conditions", "ok=true AND proofset_deleted=false")
	// Query for pdp_proofset_deletes entries where ok = TRUE and proofset_deleted = FALSE
	var proofSetDeletes []models.PDPProofsetDelete
	err := db.WithContext(ctx).
		Where("ok = ? AND proofset_deleted = ?", true, false).
		Find(&proofSetDeletes).Error
	if err != nil {
		log.Errorw("Database query for pending proof set deletes failed", "error", err)
		return fmt.Errorf("failed to select proof set deletes: %w", err)
	}

	if len(proofSetDeletes) == 0 {
		log.Debugw("No pending proof set deletions found")
		return nil
	}

	log.Infow("Found pending proof set deletions to process", "count", len(proofSetDeletes))
	// Process each proof set delete
	for i, psd := range proofSetDeletes {
		start := time.Now()
		log.Infow("Processing proof set deletion",
			"index", i+1,
			"total", len(proofSetDeletes),
			"tx_hash", psd.DeleteMessageHash,
			"service", psd.Service)

		err := processProofSetDelete(ctx, db, psd, ethClient, contractClient)
		if err != nil {
			log.Errorw("Failed to process proof set delete",
				"tx_hash", psd.DeleteMessageHash,
				"service", psd.Service,
				"error", err)
			continue
		}
		log.Infow("Successfully processed proof set deletion",
			"tx_hash", psd.DeleteMessageHash,
			"service", psd.Service,
			"duration", time.Since(start))
	}

	return nil
}

func processProofSetDelete(
	ctx context.Context,
	db *gorm.DB,
	psd models.PDPProofsetDelete,
	ethClient bind.ContractBackend,
	contractClient contract.PDP,
) error {
	txHash := psd.DeleteMessageHash
	service := psd.Service

	lg := log.With("tx_hash", txHash, "owner", service, "verifier_address", contract.Addresses().PDPVerifier.String())

	// Retrieve the tx_receipt from message_waits_eth
	lg.Debug("Retrieving transaction receipt")
	var msgWait models.MessageWaitsEth
	err := db.WithContext(ctx).
		Select("tx_receipt").
		First(&msgWait, "signed_tx_hash = ?", txHash).Error
	if err != nil {
		lg.Errorw("Failed to retrieve transaction receipt", "error", err)
		return fmt.Errorf("failed to get tx_receipt for tx %s: %w", txHash, err)
	}

	txReceiptJSON := msgWait.TxReceipt
	lg.Debugw("Successfully retrieved transaction receipt", "tx_status", msgWait.TxStatus, "tx_success", msgWait.TxSuccess)

	// Unmarshal the tx_receipt JSON into types.Receipt
	var txReceipt types.Receipt
	err = json.Unmarshal(txReceiptJSON, &txReceipt)
	if err != nil {
		lg.Error("Failed to unmarshal transaction receipt JSON", "error", err)
		return fmt.Errorf("failed to unmarshal tx_receipt for tx %s: %w", txHash, err)
	}

	// Parse the logs to extract the proofSetId and deletedLeafCount
	lg.Debug("Extracting proof set ID and deleted leaf count from transaction receipt")
	proofSetId, deletedLeafCount, err := extractProofSetDeleteInfo(&txReceipt)
	if err != nil {
		lg.Errorw("Failed to extract proof set delete info from receipt",
			"tx_hash", txHash,
			"error", err)
		return fmt.Errorf("failed to extract proof set delete info from receipt for tx %s: %w", txHash, err)
	}
	lg = lg.With("proof_set_id", proofSetId, "deleted_leaf_count", deletedLeafCount)
	lg.Debug("Extracted proof set delete info")

	// Clean up the proof set from the database
	lg.Debug("Cleaning up proof set from database")
	err = cleanupProofSet(ctx, db, proofSetId)
	if err != nil {
		lg.Errorw("Failed to cleanup proof set from database", "error", err)
		return fmt.Errorf("failed to cleanup proof set %d for tx %s: %w", proofSetId, txHash, err)
	}
	lg.Debug("Successfully cleaned up proof set record")

	// Update pdp_proofset_deletes to set proofset_deleted = TRUE
	lg.Debugw("Updating proof set deletion status")
	err = db.WithContext(ctx).
		Model(&models.PDPProofsetDelete{}).
		Where("delete_message_hash = ?", txHash).
		Update("proofset_deleted", true).Error
	if err != nil {
		lg.Errorw("Failed to update proof set deletion status", "error", err)
		return fmt.Errorf("failed to update proofset_deletes for tx %s: %w", txHash, err)
	}
	lg.Debug("Successfully updated proof set deletion status")

	lg.Infow("Successfully deleted proof set")
	return nil
}

func extractProofSetDeleteInfo(receipt *types.Receipt) (uint64, uint64, error) {
	// Get the ABI from the contract metadata
	pdpABI, err := contract.PDPVerifierMetaData()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get PDP ABI: %w", err)
	}

	// Get the event definition
	event, exists := pdpABI.Events["ProofSetDeleted"]
	if !exists {
		return 0, 0, fmt.Errorf("ProofSetDeleted event not found in ABI")
	}

	// Iterate over the logs in the receipt
	for _, vLog := range receipt.Logs {
		// Check if the log corresponds to the ProofSetDeleted event
		if len(vLog.Topics) > 0 && vLog.Topics[0] == event.ID {
			// The setId is an indexed parameter in Topics[1]
			if len(vLog.Topics) < 2 {
				return 0, 0, fmt.Errorf("log does not contain setId topic")
			}

			setIdBigInt := new(big.Int).SetBytes(vLog.Topics[1].Bytes())
			proofSetId := setIdBigInt.Uint64()

			// Parse the non-indexed parameter (deletedLeafCount) from the data
			unpacked, err := event.Inputs.Unpack(vLog.Data)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to unpack log data: %w", err)
			}

			// Extract the deletedLeafCount
			if len(unpacked) == 0 {
				return 0, 0, fmt.Errorf("no unpacked data found in log")
			}

			deletedLeafCountBigInt, ok := unpacked[0].(*big.Int)
			if !ok {
				return 0, 0, fmt.Errorf("failed to convert unpacked data to big.Int")
			}

			deletedLeafCount := deletedLeafCountBigInt.Uint64()

			return proofSetId, deletedLeafCount, nil
		}
	}

	return 0, 0, fmt.Errorf("ProofSetDeleted event not found in receipt")
}

func cleanupProofSet(ctx context.Context, db *gorm.DB, proofSetId uint64) error {
	// Start a transaction for cleanup
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete associated roots
		if err := tx.Where("proof_set_id = ?", proofSetId).Delete(&models.PDPProofsetRoot{}).Error; err != nil {
			return fmt.Errorf("failed to delete proof set roots: %w", err)
		}

		// Delete the proof set itself
		if err := tx.Where("id = ?", proofSetId).Delete(&models.PDPProofSet{}).Error; err != nil {
			return fmt.Errorf("failed to delete proof set: %w", err)
		}

		// TODO: Schedule garbage collection for orphaned blobs
		// This would involve checking PDPPieceRef.proofset_refcount and
		// scheduling cleanup when reference count reaches 0

		return nil
	})
}
