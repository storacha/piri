package service

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/filecoin-project/go-commp-utils/nonffi"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/contract"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/types"
)

// TODO return something useful here, like the transaction Hash.
func (p *PDPService) AddRoots(ctx context.Context, id uint64, request []types.RootAdd) (common.Hash, error) {
	if len(request) == 0 {
		return common.Hash{}, types.NewErrorf(types.KindInvalidInput, "must provide at least one root")
	}

	// Collect all subrootCIDs to fetch their info in a batch
	newSubroots := cid.NewSet()
	for _, addRootReq := range request {
		if !addRootReq.Root.Defined() {
			return common.Hash{}, types.NewErrorf(types.KindInvalidInput, "must provide a root CID to add")
		}

		if len(addRootReq.SubRoots) == 0 {
			return common.Hash{}, types.NewErrorf(types.KindInvalidInput, "must provide at least one subroot CID to add")
		}

		for _, subrootEntry := range addRootReq.SubRoots {
			if !subrootEntry.Defined() {
				return common.Hash{}, types.NewErrorf(types.KindInvalidInput, "subroot CID is required for each subroot")
			}
			if newSubroots.Has(subrootEntry) {
				return common.Hash{}, types.NewErrorf(types.KindInvalidInput, "subroot CID %s is duplicated", subrootEntry.String())
			}

			newSubroots.Add(subrootEntry)
		}
	}

	// Map to store subrootCID -> [pieceInfo, pdp_pieceref.id, subrootOffset]
	type SubrootInfo struct {
		PieceInfo     abi.PieceInfo
		PDPPieceRefID int64
		SubrootOffset uint64
	}

	type subrootRow struct {
		PieceCID        string `gorm:"column:piece_cid"`
		PDPPieceRefID   int64  `gorm:"column:pdp_piece_ref_id"`
		PieceRefID      int64  `gorm:"column:piece_ref"`
		PiecePaddedSize uint64 `gorm:"column:piece_padded_size"`
	}

	// Convert set to slice of string for db query
	newSubrootsList := lo.Map(newSubroots.Keys(), func(c cid.Cid, _ int) string {
		return c.String()
	})

	var rows []subrootRow
	if err := p.db.WithContext(ctx).
		Table("pdp_piecerefs as ppr").
		Select("ppr.piece_cid, ppr.id as pdp_piece_ref_id, ppr.piece_ref, pp.piece_padded_size").
		Joins("JOIN parked_piece_refs as pprf ON pprf.ref_id = ppr.piece_ref").
		Joins("JOIN parked_pieces as pp ON pp.id = pprf.piece_id").
		Where("ppr.service = ? AND ppr.piece_cid IN ?", p.name, newSubrootsList).
		Scan(&rows).Error; err != nil {
		return common.Hash{}, err
	}

	subrootInfoMap := make(map[cid.Cid]*SubrootInfo)
	currentSubroots := cid.NewSet()
	for _, r := range rows {
		// Decode the piece CID.
		decodedCID, err := cid.Decode(r.PieceCID)
		if err != nil {
			return common.Hash{}, fmt.Errorf("invalid piece CID in database: %s", r.PieceCID)
		}
		subrootInfoMap[decodedCID] = &SubrootInfo{
			PieceInfo: abi.PieceInfo{
				Size:     abi.PaddedPieceSize(r.PiecePaddedSize),
				PieceCID: decodedCID,
			},
			PDPPieceRefID: r.PDPPieceRefID,
			SubrootOffset: 0, // will be computed below
		}
		currentSubroots.Add(decodedCID)
	}

	// Ensure every requested subrootCID was found.
	if err := currentSubroots.ForEach(func(c cid.Cid) error {
		if !newSubroots.Has(c) {
			return fmt.Errorf("subroot CID %s not found or does not belong to service %s", c.String(), p.name)
		}
		return nil
	}); err != nil {
		return common.Hash{}, err
	}

	// For each AddRootRequest, validate the provided RootCID.
	for _, addReq := range request {
		// Collect pieceInfos for each subroot.
		pieceInfos := make([]abi.PieceInfo, len(addReq.SubRoots))
		var totalOffset uint64 = 0
		for i, subCID := range addReq.SubRoots {
			subInfo, exists := subrootInfoMap[subCID]
			if !exists {
				return common.Hash{}, fmt.Errorf("subroot CID %s not found in subroot info map", subCID)
			}
			// Set the offset for this subroot.
			subInfo.SubrootOffset = totalOffset
			pieceInfos[i] = subInfo.PieceInfo
			totalOffset += uint64(subInfo.PieceInfo.Size)
		}

		// Generate the unsealed CID from the collected piece infos.
		proofType := abi.RegisteredSealProof_StackedDrg64GiBV1_1
		generatedCID, err := nonffi.GenerateUnsealedCID(proofType, pieceInfos)
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to generate RootCID: %v", err)
		}
		// Compare the generated and provided CIDs.
		if !addReq.Root.Equals(generatedCID) {
			return common.Hash{}, fmt.Errorf("provided RootCID does not match generated RootCID: %s != %s", addReq.Root, generatedCID)
		}
	}

	// Step 5: Prepare the Ethereum transaction data outside the DB transaction
	// Obtain the ABI of the PDPVerifier contract
	abiData, err := contract.PDPVerifierMetaData()
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get abi data from PDPVerifierMetaData: %w", err)
	}

	// Prepare RootData array for Ethereum transaction
	// Define a Struct that matches the Solidity RootData struct
	type RootData struct {
		Root struct {
			Data []byte
		}
		RawSize *big.Int
	}

	var rootDataArray []RootData

	for _, addRootReq := range request {
		// Convert RootCID to bytes
		rootCID := addRootReq.Root

		// Get total size by summing up the sizes of subroots
		// IMPORTANT: Using padded sizes here to match what's stored in the database
		// and ensure the total is a multiple of 32 (as required by the smart contract)
		var totalSize uint64 = 0
		var totalUnpaddedSize uint64 = 0
		var prevSubrootSize = subrootInfoMap[addRootReq.SubRoots[0]].PieceInfo.Size
		for i, subrootEntry := range addRootReq.SubRoots {
			subrootInfo := subrootInfoMap[subrootEntry]
			if subrootInfo.PieceInfo.Size > prevSubrootSize {
				return common.Hash{}, fmt.Errorf("subroots must be in descending order of size, root %d %s is larger than prev subroot %s", i, subrootEntry, addRootReq.SubRoots[i-1])
			}

			prevSubrootSize = subrootInfo.PieceInfo.Size
			paddedSize := uint64(subrootInfo.PieceInfo.Size)
			unpaddedSize := uint64(subrootInfo.PieceInfo.Size.Unpadded())
			log.Debugw("Subroot size details",
				"subrootCID", subrootEntry,
				"paddedSize", paddedSize,
				"paddedSizeMod32", paddedSize%32,
				"unpaddedSize", unpaddedSize,
				"unpaddedSizeMod32", unpaddedSize%32)
			// Try using padded size instead of unpadded
			totalSize += paddedSize
			totalUnpaddedSize += unpaddedSize
		}

		// Log debug information
		log.Debugw("Root data details",
			"rootCID", addRootReq.Root,
			"cidBytesLen", len(rootCID.Bytes()),
			"totalSize", totalSize,
			"totalSizeMod32", totalSize%32,
			"totalUnpaddedSize", totalUnpaddedSize,
			"totalUnpaddedSizeMod32", totalUnpaddedSize%32,
			"subrootCount", len(addRootReq.SubRoots))

		// Prepare RootData for Ethereum transaction
		rootData := RootData{
			Root:    struct{ Data []byte }{Data: rootCID.Bytes()},
			RawSize: new(big.Int).SetUint64(totalSize),
		}

		rootDataArray = append(rootDataArray, rootData)
	}

	// Convert proofSetID to *big.Int
	proofSetID := new(big.Int).SetUint64(id)

	// Pack the method call data
	log.Infow("Adding root to proof set", "proofSetID", proofSetID)
	data, err := abiData.Pack("addRoots", proofSetID, rootDataArray, []byte{})
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to pack addRoots: %w", err)
	}

	// Prepare the transaction (nonce will be set to 0, SenderETH will assign it)
	txEth := ethtypes.NewTransaction(
		0,
		contract.Addresses().PDPVerifier,
		big.NewInt(0),
		0,
		nil,
		data,
	)

	// Step 8: Send the transaction using SenderETH
	reason := "pdp-addroots"
	txHash, err := p.sender.Send(ctx, p.address, txEth, reason)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to send transaction: %w", err)
	}

	// Step 9: Insert into message_waits_eth and pdp_proofset_roots
	if err := p.db.Transaction(func(tx *gorm.DB) error {
		// Insert into message_waits_eth
		mw := models.MessageWaitsEth{
			SignedTxHash: txHash.Hex(),
			TxStatus:     "pending",
		}
		if err := tx.WithContext(ctx).Create(&mw).Error; err != nil {
			return err
		}

		// Update proof set for initialization upon first add
		if err := tx.WithContext(ctx).
			Model(&models.PDPProofSet{}).
			Where("id = ? AND prev_challenge_request_epoch IS NULL AND challenge_request_msg_hash IS NULL AND prove_at_epoch IS NULL", proofSetID.Int64()).
			Update("init_ready", true).Error; err != nil {
			return err
		}

		// Insert into pdp_proofset_root_adds
		for addMessageIndex, addReq := range request {
			for _, subrootEntry := range addReq.SubRoots {
				subInfo := subrootInfoMap[subrootEntry]
				newRootAdd := models.PDPProofsetRootAdd{
					ProofsetID:      proofSetID.Int64(),
					Root:            addReq.Root.String(),
					AddMessageHash:  txHash.Hex(),
					AddMessageIndex: models.Ptr(int64(addMessageIndex)),
					Subroot:         subrootEntry.String(),
					SubrootOffset:   int64(subInfo.SubrootOffset),
					SubrootSize:     int64(subInfo.PieceInfo.Size),
					PDPPieceRefID:   &subInfo.PDPPieceRefID,
				}
				if err := tx.WithContext(ctx).Create(&newRootAdd).Error; err != nil {
					return err
				}
			}
		}

		// If we get here, the transaction will be committed.
		return nil
	}); err != nil {
		log.Errorw("Failed to insert into database", "error", err, "txHash", txHash.Hex(), "subroots", subrootInfoMap)
		return common.Hash{}, fmt.Errorf("failed to insert into database: %w", err)
	}
	return txHash, nil
}
