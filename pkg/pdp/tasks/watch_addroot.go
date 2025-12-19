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

	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
)

// NewWatcherRootAdd sets up the watcher for proof set root additions
func NewWatcherRootAdd(db *gorm.DB, pcs *chainsched.Scheduler, verifier smartcontracts.Verifier) error {
	if err := pcs.AddHandler(func(ctx context.Context, revert, apply *chainyypes.TipSet) error {
		err := processPendingProofSetRootAdds(ctx, db, verifier)
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
func processPendingProofSetRootAdds(ctx context.Context, db *gorm.DB, verifier smartcontracts.Verifier) error {
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
		err := processProofSetRootAdd(ctx, db, rootAdd, verifier)
		if err != nil {
			log.Warnf("Failed to process root add for tx %s: %v", rootAdd.AddMessageHash, err)
			continue
		}
	}

	return nil
}

func processProofSetRootAdd(ctx context.Context, db *gorm.DB, rootAdd models.PDPProofsetRootAdd, verifier smartcontracts.Verifier) error {
	// Retrieve the tx_receipt from message_waits_eth
	var msgWait models.MessageWaitsEth
	err := db.WithContext(ctx).
		Select("tx_receipt").
		Where("signed_tx_hash = ?", rootAdd.AddMessageHash).
		First(&msgWait).Error

	// NB(forrest): the below handles the case where the operator was unhealthy for > 16 hours
	// lotus snapshots only contain 2000 epochs of state, and therefor it is possible for a
	// receipt to be irretrievable from a lotus node when its from a block outside that time frame.

	var txReceipt *types.Receipt
	if err == nil && msgWait.TxReceipt != nil && len(msgWait.TxReceipt) > 0 {
		var receipt types.Receipt
		if err := json.Unmarshal(msgWait.TxReceipt, &receipt); err == nil {
			txReceipt = &receipt
		} else {
			log.Warnf("Failed to unmarshal tx_receipt for tx %s: %v", rootAdd.AddMessageHash, err)
		}
	} else if err != nil {
		log.Warnf("Failed to get tx_receipt from database for tx %s: %v", rootAdd.AddMessageHash, err)
	}

	// Use fallback strategy to get piece IDs
	rootIds, err := getPieceIdsWithFallback(ctx, db, verifier, rootAdd, txReceipt)
	if err != nil {
		return fmt.Errorf("failed to get piece IDs for tx %s: %w", rootAdd.AddMessageHash, err)
	}

	// Insert the root IDs
	if err := insertRootIds(ctx, db, rootAdd, rootIds); err != nil {
		return xerrors.Errorf("failed to insert root IDs for tx %s: %w", rootAdd.AddMessageHash, err)
	}

	return nil
}

func insertRootIds(
	ctx context.Context,
	db *gorm.DB,
	rootAdd models.PDPProofsetRootAdd,
	rootIds []uint64,
) error {

	// Begin a database transaction
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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

// getPieceIdsWithFallback attempts to get piece IDs from the transaction receipt first,
// and falls back to querying active pieces from the contract if the receipt is unavailable.
func getPieceIdsWithFallback(
	ctx context.Context,
	db *gorm.DB,
	verifier smartcontracts.Verifier,
	rootAdd models.PDPProofsetRootAdd,
	txReceipt *types.Receipt,
) ([]uint64, error) {
	// Try to get piece IDs from receipt first if it exists, else we request from the contract state.
	if txReceipt != nil {
		pieceIds, err := verifier.GetPieceIdsFromReceipt(txReceipt)
		if err == nil {
			return pieceIds, nil
		}
		log.Warnf("Failed to get piece IDs from receipt for tx %s: %v, falling back to getActivePieces", rootAdd.AddMessageHash, err)
	}

	// Fallback Use verifier contract getActivePieces to reconstruct piece IDs
	return getPieceIdsByMatching(ctx, db, verifier, rootAdd)
}

// getPieceIdsByMatching fetches active pieces from the contract and matches them
// with the pieces in the database by their CID to determine piece IDs.
func getPieceIdsByMatching(
	ctx context.Context,
	db *gorm.DB,
	verifier smartcontracts.Verifier,
	rootAdd models.PDPProofsetRootAdd,
) ([]uint64, error) {
	var rootAddEntries []models.PDPProofsetRootAdd
	err := db.WithContext(ctx).
		Where("proofset_id = ? AND add_message_hash = ?", rootAdd.ProofsetID, rootAdd.AddMessageHash).
		Order("add_message_index ASC, subroot_offset ASC").
		Find(&rootAddEntries).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get root add entries: %w", err)
	}

	// pieces we wanted receipts for but failed to find
	targetCIDs := make(map[string]int) // CID string -> index in result array
	for _, entry := range rootAddEntries {
		if entry.AddMessageIndex != nil {
			targetCIDs[entry.Root] = int(*entry.AddMessageIndex)
		}
	}

	// piece we get from looking in the contract, returned to caller eventually.
	pieceIDMap := make(map[string]uint64) // CID string -> piece ID

	// Need to fetch missing pieces from contract
	err = fetchPieces(ctx, verifier, rootAdd, targetCIDs, pieceIDMap)
	if err != nil {
		return nil, err
	}

	// Build and return result
	return buildPieceIdResult(rootAddEntries, pieceIDMap, targetCIDs, rootAdd)
}

// fetchPieces fetches active pieces from the contract in batches and matches them
// with target CIDs to determine piece IDs. Uses batch processing for efficiency.
func fetchPieces(
	ctx context.Context,
	verifier smartcontracts.Verifier,
	rootAdd models.PDPProofsetRootAdd,
	targetCIDs map[string]int,
	pieceIDMap map[string]uint64,
) error {
	// Find the maximum piece offset we might need
	totalPieces, err := verifier.GetActivePieceCount(ctx, big.NewInt(rootAdd.ProofsetID))
	if err != nil {
		return fmt.Errorf("failed to get active piece count: %w", err)
	}
	maxNeeded := totalPieces.Uint64()

	// Batch configuration
	offset := big.NewInt(0)
	limit := big.NewInt(500)
	proofsetID := big.NewInt(rootAdd.ProofsetID)

	log.Infof("Starting to fetch pieces for proofset %d, need %d pieces, max available: %d",
		rootAdd.ProofsetID, len(targetCIDs), maxNeeded)

	// Fetch pieces in batches until we find all needed pieces or reach the end
	for offset.Uint64() < maxNeeded {
		activePieces, err := verifier.GetActivePieces(ctx, proofsetID, offset, limit)
		if err != nil {
			return fmt.Errorf("failed to get active pieces at offset %d: %w", offset.Int64(), err)
		}

		// Process the pieces in this batch
		for i, piece := range activePieces.Pieces {
			cidStr := piece.String()
			if _, found := targetCIDs[cidStr]; found {
				pieceIDMap[cidStr] = activePieces.PieceIds[i].Uint64()
			}
		}

		log.Infof("Fetched batch at offset %d: found %d/%d pieces so far",
			offset.Int64(), len(pieceIDMap), len(targetCIDs))

		// Check if we found all pieces
		if len(pieceIDMap) == len(targetCIDs) {
			log.Infof("Found all %d pieces after fetching %d items",
				len(targetCIDs), offset.Int64()+int64(len(activePieces.PieceIds)))
			return nil
		}

		// Check if there are more pieces to fetch
		if !activePieces.HasMore {
			log.Infof("Reached end of active pieces at offset %d",
				offset.Int64()+int64(len(activePieces.PieceIds)))
			break
		}

		// Move to next batch - use actual number of pieces returned, not the limit
		actualBatchSize := big.NewInt(int64(len(activePieces.PieceIds)))
		offset.Add(offset, actualBatchSize)
	}

	// Check if we found all required pieces
	if len(pieceIDMap) < len(targetCIDs) {
		// Log which pieces weren't found for debugging
		missing := []string{}
		for cid := range targetCIDs {
			if _, found := pieceIDMap[cid]; !found {
				missing = append(missing, cid)
			}
		}
		return fmt.Errorf("failed to find all pieces: found %d/%d, missing CIDs: %v",
			len(pieceIDMap), len(targetCIDs), missing)
	}

	return nil
}

// buildPieceIdResult constructs the final result array from the piece ID map
func buildPieceIdResult(
	rootAddEntries []models.PDPProofsetRootAdd,
	pieceIDMap map[string]uint64,
	targetCIDs map[string]int,
	rootAdd models.PDPProofsetRootAdd,
) ([]uint64, error) {
	maxIndex := -1
	for _, idx := range targetCIDs {
		if idx > maxIndex {
			maxIndex = idx
		}
	}

	result := make([]uint64, maxIndex+1)
	foundCount := 0

	for _, entry := range rootAddEntries {
		if entry.AddMessageIndex == nil {
			continue
		}

		pieceID, found := pieceIDMap[entry.Root]
		if !found {
			return nil, fmt.Errorf("piece CID %s not found in active pieces for proofset %d", entry.Subroot, rootAdd.ProofsetID)
		}

		result[*entry.AddMessageIndex] = pieceID
		foundCount++
	}

	if foundCount == 0 {
		return nil, fmt.Errorf("no pieces found for tx %s", rootAdd.AddMessageHash)
	}

	return result, nil
}
