package service

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
	"github.com/storacha/piri/pkg/pdp/types"
)

// ActivePiecesProvider abstracts the chain state queries needed for repair.
// This interface allows for easier testing by mocking chain interactions.
type ActivePiecesProvider interface {
	GetActivePieces(ctx context.Context, setID *big.Int, offset *big.Int, limit *big.Int) (*smartcontracts.ActivePieces, error)
}

// RepairProofSet reconciles the on-chain state with the database state for a proof set.
// It fetches all active pieces from the chain, compares with pdp_proofset_roots,
// and repairs any gaps using metadata from pdp_proofset_root_adds.
func (p *PDPService) RepairProofSet(ctx context.Context, proofSetID uint64) (res *types.RepairResult, retErr error) {
	log.Infow("repairing proof set", "proofSetID", proofSetID)
	defer func() {
		if retErr != nil {
			log.Errorw("failed to repair proof set", "proofSetID", proofSetID, "err", retErr)
		} else {
			log.Infow("repair completed", "proofSetID", proofSetID,
				"totalOnChain", res.TotalOnChain,
				"totalInDB", res.TotalInDB,
				"totalRepaired", res.TotalRepaired,
				"totalUnrepaired", res.TotalUnrepaired)
		}
	}()

	return repairProofSetCore(ctx, p.db, p.verifierContract, proofSetID)
}

// repairProofSetCore contains the core repair logic with explicit dependencies.
// This function is extracted to enable unit testing without constructing a full PDPService.
func repairProofSetCore(
	ctx context.Context,
	db *gorm.DB,
	chainProvider ActivePiecesProvider,
	proofSetID uint64,
) (*types.RepairResult, error) {
	// Verify the proof set exists
	var proofSet models.PDPProofSet
	if err := db.WithContext(ctx).First(&proofSet, proofSetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, types.NewErrorf(types.KindNotFound, "proof set %d not found", proofSetID)
		}
		return nil, fmt.Errorf("failed to retrieve proof set: %w", err)
	}

	// Step 1: Get chain state - all active pieces from the contract
	chainPieces, err := getAllActivePiecesCore(ctx, chainProvider, proofSetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active pieces from chain: %w", err)
	}

	// Step 2: Get database state - distinct roots from pdp_proofset_roots
	var dbRoots []string
	if err := db.WithContext(ctx).
		Model(&models.PDPProofsetRoot{}).
		Where("proofset_id = ?", proofSetID).
		Distinct("root").
		Pluck("root", &dbRoots).Error; err != nil {
		return nil, fmt.Errorf("failed to get roots from database: %w", err)
	}

	// Create a set of existing root CIDs for fast lookup
	dbRootSet := make(map[string]struct{}, len(dbRoots))
	for _, root := range dbRoots {
		dbRootSet[root] = struct{}{}
	}

	// Step 3: Find gaps - pieces on chain but not in database
	type missingRoot struct {
		CID     string
		PieceID uint64
	}
	var missingRoots []missingRoot

	for cidStr, pieceID := range chainPieces {
		if _, exists := dbRootSet[cidStr]; !exists {
			missingRoots = append(missingRoots, missingRoot{
				CID:     cidStr,
				PieceID: pieceID,
			})
		}
	}

	result := &types.RepairResult{
		TotalOnChain:      len(chainPieces),
		TotalInDB:         len(dbRoots),
		RepairedEntries:   []types.RepairedEntry{},
		UnrepairedEntries: []types.UnrepairedEntry{},
	}

	// Step 4: If no gaps, return success
	if len(missingRoots) == 0 {
		log.Infow("no repair needed - chain and database are in sync",
			"proofSetID", proofSetID,
			"totalOnChain", len(chainPieces),
			"totalInDB", len(dbRoots))
		return result, nil
	}

	log.Infow("found gaps to repair",
		"proofSetID", proofSetID,
		"missingCount", len(missingRoots))

	// Step 5: For each missing root, try to repair using pdp_proofset_root_adds
	for _, missing := range missingRoots {
		repaired, err := repairSingleRootCore(ctx, db, proofSetID, missing.CID, missing.PieceID)
		if err != nil {
			log.Warnw("failed to repair root",
				"proofSetID", proofSetID,
				"rootCID", missing.CID,
				"pieceID", missing.PieceID,
				"err", err)
			result.UnrepairedEntries = append(result.UnrepairedEntries, types.UnrepairedEntry{
				RootCID: missing.CID,
				RootID:  missing.PieceID,
				Reason:  err.Error(),
			})
			continue
		}

		result.RepairedEntries = append(result.RepairedEntries, types.RepairedEntry{
			RootCID:  missing.CID,
			RootID:   missing.PieceID,
			Subroots: repaired,
		})
	}

	result.TotalRepaired = len(result.RepairedEntries)
	result.TotalUnrepaired = len(result.UnrepairedEntries)

	return result, nil
}

// getAllActivePiecesCore fetches all active pieces from the chain with pagination.
// Returns a map of CID string to PieceID.
// This function is extracted to enable unit testing with a mock chain provider.
func getAllActivePiecesCore(ctx context.Context, chainProvider ActivePiecesProvider, proofSetID uint64) (map[string]uint64, error) {
	setID := big.NewInt(int64(proofSetID))

	log.Infow("fetching active pieces from chain", "proofSetID", proofSetID)

	result := make(map[string]uint64)

	// Paginate through all active pieces
	offset := big.NewInt(0)
	limit := big.NewInt(100) // Fetch 100 at a time

	for {
		activePieces, err := chainProvider.GetActivePieces(ctx, setID, offset, limit)
		if err != nil {
			return nil, fmt.Errorf("failed to get active pieces at offset %d: %w", offset.Uint64(), err)
		}

		// Add pieces to result map
		for i, pieceCID := range activePieces.Pieces {
			cidStr := pieceCID.String()
			pieceID := activePieces.PieceIds[i].Uint64()
			result[cidStr] = pieceID
		}

		log.Debugw("fetched page of active pieces",
			"proofSetID", proofSetID,
			"offset", offset.Uint64(),
			"pageSize", len(activePieces.Pieces),
			"totalFetched", len(result),
			"hasMore", activePieces.HasMore)

		// Check if we've fetched all pieces
		if !activePieces.HasMore {
			break
		}

		// Move to next page
		offset = new(big.Int).Add(offset, limit)
	}

	log.Infow("finished fetching active pieces from chain",
		"proofSetID", proofSetID,
		"totalCount", len(result))

	return result, nil
}

// repairSingleRootCore attempts to repair a single missing root using metadata from pdp_proofset_root_adds.
// Returns the number of subroots repaired.
// This function is extracted to enable unit testing with a mock database.
func repairSingleRootCore(ctx context.Context, db *gorm.DB, proofSetID uint64, rootCID string, pieceID uint64) (int, error) {
	// Look up metadata in pdp_proofset_root_adds
	var rootAdds []models.PDPProofsetRootAdd
	if err := db.WithContext(ctx).
		Where("proofset_id = ? AND root = ?", proofSetID, rootCID).
		Order("subroot_offset ASC").
		Find(&rootAdds).Error; err != nil {
		return 0, fmt.Errorf("failed to query pdp_proofset_root_adds: %w", err)
	}

	if len(rootAdds) == 0 {
		return 0, fmt.Errorf("metadata not found in pdp_proofset_root_adds")
	}

	// Collect unique message hashes to mark as confirmed later
	messageHashes := make(map[string]struct{})

	// Perform the repair in a transaction
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Insert entries into pdp_proofset_roots
		// Use ON CONFLICT DO NOTHING to handle race conditions with WatcherRootAdd
		for _, entry := range rootAdds {
			// Nil check for AddMessageIndex
			var addMessageIndex int64
			if entry.AddMessageIndex != nil {
				addMessageIndex = *entry.AddMessageIndex
			}

			root := models.PDPProofsetRoot{
				ProofsetID:      entry.ProofsetID,
				RootID:          int64(pieceID),
				SubrootOffset:   entry.SubrootOffset,
				Root:            entry.Root,
				AddMessageHash:  entry.AddMessageHash,
				AddMessageIndex: addMessageIndex,
				Subroot:         entry.Subroot,
				SubrootSize:     entry.SubrootSize,
				PDPPieceRefID:   entry.PDPPieceRefID,
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&root).Error; err != nil {
				return fmt.Errorf("failed to insert pdp_proofset_root: %w", err)
			}

			// Track message hash for later confirmation
			messageHashes[entry.AddMessageHash] = struct{}{}
		}

		// Delete entries from pdp_proofset_root_adds
		if err := tx.Where("proofset_id = ? AND root = ?", proofSetID, rootCID).
			Delete(&models.PDPProofsetRootAdd{}).Error; err != nil {
			return fmt.Errorf("failed to delete from pdp_proofset_root_adds: %w", err)
		}

		// Mark associated message_waits_eth entries as confirmed
		// This prevents WatcherEth from continuing to poll for receipts that Lotus doesn't have
		for msgHash := range messageHashes {
			txSuccess := true
			if err := tx.Model(&models.MessageWaitsEth{}).
				Where("signed_tx_hash = ?", msgHash).
				Where("tx_status = ?", "pending").
				Updates(map[string]interface{}{
					"tx_status":  "confirmed",
					"tx_success": &txSuccess,
				}).Error; err != nil {
				return fmt.Errorf("failed to update message_waits_eth for %s: %w", msgHash, err)
			}
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	log.Infow("repaired root",
		"proofSetID", proofSetID,
		"rootCID", rootCID,
		"pieceID", pieceID,
		"subrootsRepaired", len(rootAdds),
		"messagesMarkedConfirmed", len(messageHashes))

	return len(rootAdds), nil
}
