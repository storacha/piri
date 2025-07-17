package service

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/filecoin-project/go-commp-utils/nonffi"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/contract"
	"github.com/storacha/piri/pkg/pdp/service/models"
)

type AddRootRequest struct {
	RootCID     string
	SubrootCIDs []string
}

// TODO return something useful here, like the transaction Hash.
func (p *PDPService) ProofSetAddRoot(ctx context.Context, id int64, request []AddRootRequest) (interface{}, error) {
	if len(request) == 0 {
		return nil, fmt.Errorf("at least one root must be provided")
	}

	// Collect all subrootCIDs to fetch their info in a batch
	subrootCIDsSet := make(map[string]struct{})
	for _, addRootReq := range request {
		if addRootReq.RootCID == "" {
			return nil, fmt.Errorf("rootCID is required for each root")
		}

		if len(addRootReq.SubrootCIDs) == 0 {
			return nil, fmt.Errorf("at least one subroot is required per root")
		}

		for _, subrootEntry := range addRootReq.SubrootCIDs {
			if subrootEntry == "" {
				return nil, fmt.Errorf("subrootCid is required for each subroot")
			}
			if _, exists := subrootCIDsSet[subrootEntry]; exists {
				return nil, fmt.Errorf("duplicate subrootCid in request")
			}

			subrootCIDsSet[subrootEntry] = struct{}{}
		}
	}

	// Convert set to slice
	subrootCIDsList := make([]string, 0, len(subrootCIDsSet))
	for cidStr := range subrootCIDsSet {
		subrootCIDsList = append(subrootCIDsList, cidStr)
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

	subrootInfoMap := make(map[string]*SubrootInfo)

	var rows []subrootRow
	if err := p.db.WithContext(ctx).
		Table("pdp_piecerefs as ppr").
		Select("ppr.piece_cid, ppr.id as pdp_piece_ref_id, ppr.piece_ref, pp.piece_padded_size").
		Joins("JOIN parked_piece_refs as pprf ON pprf.ref_id = ppr.piece_ref").
		Joins("JOIN parked_pieces as pp ON pp.id = pprf.piece_id").
		Where("ppr.service = ? AND ppr.piece_cid IN ?", p.name, subrootCIDsList).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	// Start a GORM transaction.
	foundSubroots := make(map[string]struct{})
	for _, r := range rows {
		// Decode the piece CID.
		decodedCID, err := cid.Decode(r.PieceCID)
		if err != nil {
			return nil, fmt.Errorf("invalid piece CID in database: %s", r.PieceCID)
		}
		pieceInfo := abi.PieceInfo{
			Size:     abi.PaddedPieceSize(r.PiecePaddedSize),
			PieceCID: decodedCID,
		}
		subrootInfoMap[r.PieceCID] = &SubrootInfo{
			PieceInfo:     pieceInfo,
			PDPPieceRefID: r.PDPPieceRefID,
			SubrootOffset: 0, // will be computed below
		}
		foundSubroots[r.PieceCID] = struct{}{}
	}

	// Ensure every requested subrootCID was found.
	for _, cidStr := range subrootCIDsList {
		if _, ok := foundSubroots[cidStr]; !ok {
			return nil, fmt.Errorf("subroot CID %s not found or does not belong to service %s", cidStr, p.name)
		}
	}

	// For each AddRootRequest, validate the provided RootCID.
	for _, addReq := range request {
		// Collect pieceInfos for each subroot.
		pieceInfos := make([]abi.PieceInfo, len(addReq.SubrootCIDs))
		var totalOffset uint64 = 0
		for i, subCID := range addReq.SubrootCIDs {
			subInfo, exists := subrootInfoMap[subCID]
			if !exists {
				return nil, fmt.Errorf("subroot CID %s not found in subroot info map", subCID)
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
			return nil, fmt.Errorf("failed to generate RootCID: %v", err)
		}
		// Decode the provided RootCID.
		providedCID, err := cid.Decode(addReq.RootCID)
		if err != nil {
			return nil, fmt.Errorf("invalid provided RootCID: %v", err)
		}
		// Compare the generated and provided CIDs.
		if !providedCID.Equals(generatedCID) {
			return nil, fmt.Errorf("provided RootCID does not match generated RootCID: %s != %s", providedCID, generatedCID)
		}
	}

	// Step 5: Prepare the Ethereum transaction data outside the DB transaction
	// Obtain the ABI of the PDPVerifier contract
	abiData, err := contract.PDPVerifierMetaData()
	if err != nil {
		return nil, fmt.Errorf("failed to get abi data from PDPVerifierMetaData: %w", err)
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
		rootCID, err := cid.Decode(addRootReq.RootCID)
		if err != nil {
			return nil, fmt.Errorf("invalid RootCID: %w", err)
		}

		// Get total size by summing up the sizes of subroots
		// IMPORTANT: Using padded sizes here to match what's stored in the database
		// and ensure the total is a multiple of 32 (as required by the smart contract)
		var totalSize uint64 = 0
		var totalUnpaddedSize uint64 = 0
		var prevSubrootSize = subrootInfoMap[addRootReq.SubrootCIDs[0]].PieceInfo.Size
		for i, subrootEntry := range addRootReq.SubrootCIDs {
			subrootInfo := subrootInfoMap[subrootEntry]
			if subrootInfo.PieceInfo.Size > prevSubrootSize {
				return nil, fmt.Errorf("subroots must be in descending order of size, root %d %s is larger than prev subroot %s", i, subrootEntry, addRootReq.SubrootCIDs[i-1])
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
			"rootCID", addRootReq.RootCID,
			"cidBytesLen", len(rootCID.Bytes()),
			"totalSize", totalSize,
			"totalSizeMod32", totalSize%32,
			"totalUnpaddedSize", totalUnpaddedSize,
			"totalUnpaddedSizeMod32", totalUnpaddedSize%32,
			"subrootCount", len(addRootReq.SubrootCIDs))

		// Prepare RootData for Ethereum transaction
		rootData := RootData{
			Root:    struct{ Data []byte }{Data: rootCID.Bytes()},
			RawSize: new(big.Int).SetUint64(totalSize),
		}

		rootDataArray = append(rootDataArray, rootData)
	}

	// Convert proofSetID to *big.Int
	proofSetID := new(big.Int).SetUint64(uint64(id))

	// Pack the method call data
	data, err := abiData.Pack("addRoots", proofSetID, rootDataArray, []byte{})
	if err != nil {
		return nil, fmt.Errorf("failed to pack addRoots: %w", err)
	}

	// Prepare the transaction (nonce will be set to 0, SenderETH will assign it)
	txEth := types.NewTransaction(
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
		return nil, fmt.Errorf("failed to send transaction: %w", err)
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
			for _, subrootEntry := range addReq.SubrootCIDs {
				subInfo := subrootInfoMap[subrootEntry]
				newRootAdd := models.PDPProofsetRootAdd{
					ProofsetID:      proofSetID.Int64(),
					Root:            addReq.RootCID,
					AddMessageHash:  txHash.Hex(),
					AddMessageIndex: models.Ptr(int64(addMessageIndex)),
					Subroot:         subrootEntry,
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
		return nil, fmt.Errorf("failed to insert into database: %w", err)
	}
	return nil, nil
}
