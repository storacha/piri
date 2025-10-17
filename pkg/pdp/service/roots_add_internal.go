package service

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	commcid "github.com/filecoin-project/go-fil-commcid"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/storacha/filecoin-services/go/eip712"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
	"github.com/storacha/piri/pkg/pdp/types"
)

// AddRootsInternal is an internal version of AddRoots that accepts a specific firstAdded value
// This allows the coordinator to control the nextPieceId value used for signature generation
func (p *PDPService) AddRootsInternal(ctx context.Context, id uint64, request []types.RootAdd, firstAdded uint64) (res common.Hash, retErr error) {
	log.Infow("adding roots (internal)", "id", id, "request", request, "firstAdded", firstAdded)
	defer func() {
		if retErr != nil {
			log.Errorw("failed to add roots (internal)", "id", id, "request", request, "firstAdded", firstAdded, "err", retErr)
		} else {
			log.Infow("added roots (internal)", "id", id, "request", request, "firstAdded", firstAdded, "response", res)
		}
	}()

	// Check if the provider is both registered and approved
	if err := p.RequireProviderApproved(ctx); err != nil {
		return common.Hash{}, err
	}

	// Check if the proof set exists
	var proofSet models.PDPProofSet
	if err := p.db.WithContext(ctx).Where("id = ?", id).First(&proofSet).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return common.Hash{}, types.NewErrorf(types.KindNotFound,
				"proof set %d does not exist. Must create a proof set first using CreateProofSet before adding roots", id)
		}
		return common.Hash{}, fmt.Errorf("failed to check if proof set exists: %w", err)
	}

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

	// Map to store subrootCID -> [pieceInfo, pdp_pieceref.id, subrootOffset, rawSize]
	type SubrootInfo struct {
		PieceInfo     abi.PieceInfo
		PDPPieceRefID int64
		SubrootOffset uint64
		RawSize       uint64 // Actual unpadded data size (not derived from padded size)
	}

	type subrootRow struct {
		PieceCID        string `gorm:"column:piece_cid"`
		PDPPieceRefID   int64  `gorm:"column:pdp_piece_ref_id"`
		PieceRefID      int64  `gorm:"column:piece_ref"`
		PiecePaddedSize uint64 `gorm:"column:piece_padded_size"`
		PieceRawSize    int64  `gorm:"column:piece_raw_size"`
	}

	// Convert set to slice of string for db query
	newSubrootsList := lo.Map(newSubroots.Keys(), func(c cid.Cid, _ int) string {
		return c.String()
	})

	var rows []subrootRow
	if err := p.db.WithContext(ctx).
		Table("pdp_piecerefs as ppr").
		Select("ppr.piece_cid, ppr.id as pdp_piece_ref_id, ppr.piece_ref, pp.piece_padded_size, pp.piece_raw_size").
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
			RawSize:       uint64(r.PieceRawSize),
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
		generatedCID, err := GenerateUnsealedCID(proofType, pieceInfos)
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to generate RootCID: %w", err)
		}

		// turn the uploaded roots into PieceCIDV1
		providedPieceCidV1, err := asPieceCIDv1(addReq.Root.String())
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to generate PieceCIDV1 for request: %w", err)
		}

		// Compare the generated and provided CIDs.
		if !providedPieceCidV1.Equals(generatedCID) {
			return common.Hash{}, fmt.Errorf("provided RootCID does not match generated RootCID: %s (v1 %s) != %s",
				addReq.Root, providedPieceCidV1, generatedCID)
		}
	}

	// Step 5: Prepare the Ethereum transaction data
	// Obtain the ABI of the PDPVerifier contract
	abiData, err := p.verifierContract.GetABI()
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get abi data from PDPVerifierMetaData: %w", err)
	}

	// Prepare PieceData array for Ethereum transaction
	var pieceDataArray []smartcontracts.CidsCid

	for _, addRootReq := range request {
		// Convert RootCID to bytes
		rootCID := addRootReq.Root

		_, rawSize, err := commcid.PieceCidV1FromV2(rootCID)
		if err != nil {
			return common.Hash{}, fmt.Errorf("invalid PieceCIDV2: %w", err)
		}
		height, _, err := commcid.PayloadSizeToV1TreeHeightAndPadding(rawSize)
		if err != nil {
			return common.Hash{}, fmt.Errorf("computing height and padding: %w", err)
		}
		if height > 50 {
			return common.Hash{}, fmt.Errorf("invalid height: %d", height)
		}

		// Get total size by summing up the sizes of subroots
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
			unpaddedSize := subrootInfo.RawSize

			log.Infow("Subroot size details",
				"subrootCID", subrootEntry,
				"paddedSize", paddedSize,
				"unpaddedSize", unpaddedSize,
				"rawSizeFromDB", subrootInfo.RawSize)
			totalSize += paddedSize
			totalUnpaddedSize += unpaddedSize
		}

		// Log debug information
		log.Infow("Root data details",
			"rootCID", addRootReq.Root,
			"totalSize", totalSize,
			"totalUnpaddedSize", totalUnpaddedSize,
			"subrootCount", len(addRootReq.SubRoots))

		// Check if the rawSize in the PieceCIDv2 matches the totalUnpaddedSize of the subPieces
		if rawSize != totalUnpaddedSize {
			log.Warnw("PieceCIDv2 embedded rawSize mismatch - will reconstruct with correct size",
				"embeddedRawSize", rawSize,
				"calculatedTotalUnpaddedSize", totalUnpaddedSize,
				"rootCID", rootCID.String())
		}

		// Defensively reconstruct the PieceCIDv2 to ensure it has the correct embedded size
		commitment, extractedPayloadSize, err := commcid.PieceCidV2ToDataCommitment(rootCID)
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to extract commitment from PieceCIDv2: %w", err)
		}

		// Additional validation
		if extractedPayloadSize != totalUnpaddedSize {
			log.Warnw("PieceCIDv2 embedded size mismatch - will reconstruct with correct size",
				"extractedPayloadSize", extractedPayloadSize,
				"totalUnpaddedSize", totalUnpaddedSize,
				"rootCID", rootCID.String())
		}
		reconstructedPieceCidV2, err := commcid.DataCommitmentToPieceCidv2(commitment, totalUnpaddedSize)
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to reconstruct PieceCIDv2: %w", err)
		}

		// Use the reconstructed CIDv2 to ensure correct size is embedded
		rootCID = reconstructedPieceCidV2

		// Prepare RootData for Ethereum transaction using the generated binding type
		rootData := smartcontracts.CidsCid{
			Data: rootCID.Bytes(),
		}

		pieceDataArray = append(pieceDataArray, rootData)
	}

	// Convert proofSetID to *big.Int for contract calls
	proofSetID := new(big.Int).SetUint64(id)

	// Get dataset info to obtain the clientDataSetId
	datasetInfo, err := p.serviceContract.GetDataSet(ctx, proofSetID)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get dataset info: %w", err)
	}

	// Use the provided firstAdded value instead of fetching from chain
	nextPieceId := new(big.Int).SetUint64(firstAdded)

	log.Infow("AddPieces signing parameters (internal)",
		"proofSetID", proofSetID,
		"clientDataSetId", datasetInfo.ClientDataSetId,
		"datasetPayer", datasetInfo.Payer,
		"configuredPayer", smartcontracts.PayerAddress,
		"firstAdded", nextPieceId,
		"pieceCount", len(pieceDataArray))

	// Convert pieceDataArray to [][]byte for signing
	pieceDataBytes := make([][]byte, len(pieceDataArray))
	for i, piece := range pieceDataArray {
		pieceDataBytes[i] = piece.Data
	}

	// Prepare metadata arrays (one empty array per piece for now)
	metadata := make([][]eip712.MetadataEntry, len(pieceDataArray))
	for i := range metadata {
		metadata[i] = []eip712.MetadataEntry{}
	}

	// Request a signature for adding pieces from the signing service
	// Use the provided firstAdded value for signature generation
	signature, err := p.signingService.SignAddPieces(ctx,
		datasetInfo.ClientDataSetId, // Use FilecoinWarmStorageService clientDataSetId
		nextPieceId,                 // Use the provided firstAdded value
		pieceDataBytes,
		metadata,
	)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to sign AddPieces: %w", err)
	}

	// Encode the extraData with signature and metadata
	extraDataBytes, err := p.edc.EncodeAddPiecesExtraData(signature, metadata)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to encode extraData: %w", err)
	}

	// Pack the method call data
	log.Infow("AddPieces contract parameters (internal)",
		"proofSetID", proofSetID,
		"pieceCount", len(pieceDataArray),
		"firstPieceCID", hex.EncodeToString(pieceDataArray[0].Data))

	// listener must be empty address for datasets that already exist, thus 3rd argument.
	data, err := abiData.Pack("addPieces", proofSetID, common.Address{}, pieceDataArray, extraDataBytes)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to pack addRoots: %w", err)
	}

	// Prepare the transaction (nonce will be set to 0, SenderETH will assign it)
	txEth := ethtypes.NewTransaction(
		0,
		smartcontracts.Addresses().Verifier,
		big.NewInt(0),
		0,
		nil,
		data,
	)

	// Send the transaction using SenderETH
	reason := "pdp-addroots"
	txHash, err := p.sender.Send(ctx, p.address, txEth, reason)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to send transaction: %w", err)
	}

	// Insert into message_waits_eth and pdp_proofset_roots
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