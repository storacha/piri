package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"golang.org/x/xerrors"
	"gorm.io/gorm"

	chainyypes "github.com/filecoin-project/lotus/chain/types"

	"github.com/storacha/storage/pkg/pdp/scheduler"
	"github.com/storacha/storage/pkg/pdp/service/contract"
	"github.com/storacha/storage/pkg/pdp/service/models"
)

// Structures to represent database records
type ProofSetRootAdd struct {
	ProofSet       uint64 `db:"proofset"`
	AddMessageHash string `db:"add_message_hash"`
}

// RootAddEntry represents entries from pdp_proofset_root_adds
type RootAddEntry struct {
	ProofSet        uint64 `db:"proofset"`
	Root            string `db:"root"`
	AddMessageHash  string `db:"add_message_hash"`
	AddMessageIndex uint64 `db:"add_message_index"`
	Subroot         string `db:"subroot"`
	SubrootOffset   int64  `db:"subroot_offset"`
	SubrootSize     int64  `db:"subroot_size"`
	PDPPieceRefID   int64  `db:"pdp_pieceref"`
	AddMessageOK    *bool  `db:"add_message_ok"`
	PDPProofSetID   uint64 `db:"proofset"`
}

// NewWatcherRootAdd sets up the watcher for proof set root additions
func NewWatcherRootAdd(db *gorm.DB, pcs *scheduler.Chain) error {
	if err := pcs.AddHandler(func(ctx context.Context, revert, apply *chainyypes.TipSet) error {
		err := processPendingProofSetRootAdds(ctx, db)
		if err != nil {
			log.Errorf("Failed to process pending proof set root adds: %v", err)
		}

		return nil
	}); err != nil {
		return err
	}
	return nil
}

// processPendingProofSetRootAdds processes root additions that have been confirmed on-chain
func processPendingProofSetRootAdds(ctx context.Context, db *gorm.DB) error {
	// Query for pdp_proofset_root_adds entries where add_message_ok = TRUE
	var rootAdds []models.PDPProofsetRootAdd
	err := db.WithContext(ctx).
		Distinct("proofset_id", "add_message_hash").
		Where("add_message_ok = ?", true).
		Find(&rootAdds).Error
	if err != nil {
		return fmt.Errorf("failed to select proof set root adds: %w", err)
	}

	if len(rootAdds) == 0 {
		// No pending root adds
		return nil
	}

	// Process each root addition
	for _, rootAdd := range rootAdds {
		err := processProofSetRootAdd(ctx, db, rootAdd)
		if err != nil {
			log.Warnf("Failed to process root add for tx %s: %v", rootAdd.AddMessageHash, err)
			continue
		}
	}

	return nil
}

func processProofSetRootAdd(ctx context.Context, db *gorm.DB, rootAdd models.PDPProofsetRootAdd) error {
	// Retrieve the tx_receipt from message_waits_eth
	var msgWait models.MessageWaitsEth
	err := db.WithContext(ctx).
		Select("tx_receipt").
		Where("signed_tx_hash = ?", rootAdd.AddMessageHash).
		First(&msgWait).Error
	if err != nil {
		return fmt.Errorf("failed to get tx_receipt for tx %s: %w", rootAdd.AddMessageHash, err)
	}
	txReceiptJSON := msgWait.TxReceipt

	// Unmarshal the tx_receipt JSON into types.Receipt
	var txReceipt types.Receipt
	err = json.Unmarshal(txReceiptJSON, &txReceipt)
	if err != nil {
		return xerrors.Errorf("failed to unmarshal tx_receipt for tx %s: %w", rootAdd.AddMessageHash, err)
	}

	// Parse the logs to extract root IDs and other data
	err = extractAndInsertRootsFromReceipt(ctx, db, &txReceipt, rootAdd)
	if err != nil {
		return xerrors.Errorf("failed to extract roots from receipt for tx %s: %w", rootAdd.AddMessageHash, err)
	}

	return nil
}

func extractAndInsertRootsFromReceipt(ctx context.Context, db *gorm.DB, receipt *types.Receipt, rootAdd models.PDPProofsetRootAdd) error {
	// Get the ABI from the contract metadata
	pdpABI, err := contract.PDPVerifierMetaData.GetAbi()
	if err != nil {
		return fmt.Errorf("failed to get PDP ABI: %w", err)
	}

	// Get the event definition
	event, exists := pdpABI.Events["RootsAdded"]
	if !exists {
		return fmt.Errorf("RootsAdded event not found in ABI")
	}

	var rootIds []uint64
	eventFound := false

	// Iterate over the logs in the receipt
	for _, vLog := range receipt.Logs {
		// Check if the log corresponds to the RootsAdded event
		if len(vLog.Topics) > 0 && vLog.Topics[0] == event.ID {
			// The setId is an indexed parameter in Topics[1], but we don't need it here
			// as we already have the proofset ID from the database

			// Parse the non-indexed parameter (rootIds array) from the data
			unpacked, err := event.Inputs.Unpack(vLog.Data)
			if err != nil {
				return fmt.Errorf("failed to unpack log data: %w", err)
			}

			// Extract the rootIds array
			if len(unpacked) == 0 {
				return fmt.Errorf("no unpacked data found in log")
			}

			// Convert the unpacked rootIds ([]interface{} containing *big.Int) to []uint64
			bigIntRootIds, ok := unpacked[0].([]*big.Int)
			if !ok {
				return fmt.Errorf("failed to convert unpacked data to array")
			}

			rootIds = make([]uint64, len(bigIntRootIds))
			for i := range bigIntRootIds {
				rootIds[i] = bigIntRootIds[i].Uint64()
			}

			eventFound = true
			// We found the event, so we can break the loop
			break
		}
	}

	if !eventFound {
		return fmt.Errorf("RootsAdded event not found in receipt")
	}

	// Now we have the firstAdded rootId, proceed with database operations

	// Begin a database transaction
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Fetch the entries from pdp_proofset_root_adds
		var rootAddEntries []models.PDPProofsetRootAdd
		err := tx.Where("proofset_id = ? AND add_message_hash = ?", rootAdd.ProofsetID, rootAdd.AddMessageHash).
			Order("add_message_index ASC, subroot_offset ASC").
			Find(&rootAddEntries).Error
		if err != nil {
			return fmt.Errorf("failed to select from pdp_proofset_root_adds: %w", err)
		}

		// For each entry, use the corresponding rootId from the event
		for _, entry := range rootAddEntries {
			if *entry.AddMessageIndex >= int64(len(rootIds)) {
				return fmt.Errorf("index out of bounds: entry index %d exceeds rootIds length %d",
					entry.AddMessageIndex, len(rootIds))
			}

			rootId := rootIds[*entry.AddMessageIndex]
			// Insert into pdp_proofset_roots
			root := models.PDPProofsetRoot{
				ProofsetID:      entry.ProofsetID,
				Root:            entry.Root,
				RootID:          int64(rootId),
				Subroot:         entry.Subroot,
				SubrootOffset:   entry.SubrootOffset,
				SubrootSize:     entry.SubrootSize,
				PDPPieceRefID:   entry.PDPPieceRefID,
				AddMessageHash:  entry.AddMessageHash,
				AddMessageIndex: *entry.AddMessageIndex,
			}
			err := tx.Create(&root).Error
			if err != nil {
				return fmt.Errorf("failed to insert into pdp_proofset_roots: %w", err)
			}
		}

		// Delete from pdp_proofset_root_adds
		err = tx.Where("proofset_id = ? AND add_message_hash = ?", rootAdd.ProofsetID, rootAdd.AddMessageHash).
			Delete(&models.PDPProofsetRootAdd{}).Error
		if err != nil {
			return fmt.Errorf("failed to delete from pdp_proofset_root_adds: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to process root additions in DB: %w", err)
	}

	return nil
}
