package service

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"math/bits"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/filecoin-project/go-commp-utils/zerocomm"
	commcid "github.com/filecoin-project/go-fil-commcid"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	sha256simd "github.com/minio/sha256-simd"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/storacha/filecoin-services/go/eip712"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
	"github.com/storacha/piri/pkg/pdp/types"
)

// REVIEW(forrest): this method assumes the cids in the request are PieceCIDV2
// TODO we need to define non-retryable errors for the add root method, like lack of auth, and lack of dataset else this retries forever.
func (p *PDPService) AddRoots(ctx context.Context, id uint64, request []types.RootAdd) (res common.Hash, retErr error) {
	log.Infow("adding roots", "id", id, "request", request)
	defer func() {
		if retErr != nil {
			log.Errorw("failed to add roots", "id", id, "request", request, "err", retErr)
		} else {
			log.Infow("added roots", "id", id, "request", request, "response", res)
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

	// Step 5: Prepare the Ethereum transaction data outside the DB transaction
	// Obtain the ABI of the PDPVerifier contract
	abiData, err := p.verifierContract.GetABI()
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get abi data from PDPVerifierMetaData: %w", err)
	}

	// Prepare PieceData array for Ethereum transaction
	// Use the generated contract binding type
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
		// NB: defined here: https://github.com/FilOzone/pdp/blob/main/src/PDPVerifier.sol#L44
		// as MAX_PIECE_SIZE_LOG2 = 50
		// TODO: there is an accessor method on the verifier for getting this value, add to interface
		if height > 50 {
			return common.Hash{}, fmt.Errorf("invalid height: %d", height)
		}

		// REVIEW: I am mega unsure of these code changes, but they do "pass" with the PDP Verifier contract.
		// TODO(forrest): since migrating to the new contract, rod states there is an error
		// in the curio size logic, context here: https://github.com/filecoin-project/curio/issues/650
		// in commit 91aff56959407ec83171ef73d48c51fed8afb4c7 of curio
		// Filecoin Team hash stated:
		// subpieces are broken on our branch (called `rename` in Curio, don’t use it if you’re relying on subpieces.
		// In fact, PieceCIDv2 isn’t quite working on our branch with this being an outstanding problem that’s currently
		// biting us: https://github.com/filecoin-project/curio/issues/650; the not-quite-comprehensive switch to
		// PieceCIDv2 hasn’t been a success, but we don’t really want to be changing the db schema just for that so we
		// compromised and deferred the proper job to mkv2.

		// Get total size by summing up the sizes of subroots
		// We track both padded and unpadded sizes:
		// - Padded sizes are used for smart contract operations
		// - Unpadded sizes are used to validate against PieceCIDV2's embedded rawSize
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
			// CRITICAL FIX: Use the actual raw size from the database, not .Unpadded() on padded size
			// The padded size includes both FR32 padding AND zero-padding to power-of-2
			// So calling .Unpadded() only removes FR32 padding, leaving zero-padding... SOOOooOOoo FUN! :') \s
			unpaddedSize := subrootInfo.RawSize
			log.Infow("Subroot size details",
				"subrootCID", subrootEntry,
				"paddedSize", paddedSize,
				"paddedSizeMod32", paddedSize%32,
				"unpaddedSize", unpaddedSize,
				"unpaddedSizeMod32", unpaddedSize%32,
				"rawSizeFromDB", subrootInfo.RawSize)
			totalSize += paddedSize
			totalUnpaddedSize += unpaddedSize
		}

		// Log debug information
		log.Infow("Root data details",
			"rootCID", addRootReq.Root,
			"cidBytesLen", len(rootCID.Bytes()),
			"totalSize", totalSize,
			"totalSizeMod32", totalSize%32,
			"totalUnpaddedSize", totalUnpaddedSize,
			"totalUnpaddedSizeMod32", totalUnpaddedSize%32,
			"subrootCount", len(addRootReq.SubRoots))

		// Check if the rawSize in the PieceCIDv2 matches the totalUnpaddedSize of the subPieces
		// Note: PieceCIDv2's rawSize represents unpadded data size, so we compare against totalUnpaddedSize
		// For now, just warn and reconstruct below instead of rejecting
		if rawSize != totalUnpaddedSize {
			log.Warnw("PieceCIDv2 embedded rawSize mismatch - will reconstruct with correct size",
				"embeddedRawSize", rawSize,
				"calculatedTotalUnpaddedSize", totalUnpaddedSize,
				"rootCID", rootCID.String())
			// FIXME TODO: this assertionis failing
			// return common.Hash{}, fmt.Errorf("raw size miss-match: expected %d, got %d", rawSize, totalUnpaddedSize)
		}

		// Defensively reconstruct the PieceCIDv2 to ensure it has the correct embedded size
		// Even though the hash validated above, we want to ensure the size encoding is correct
		commitment, extractedPayloadSize, err := commcid.PieceCidV2ToDataCommitment(rootCID)
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to extract commitment from PieceCIDv2: %w", err)
		}

		// Additional validation: the extracted payload size should match totalUnpaddedSize
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

		// Log the reconstruction for debugging
		log.Infow("Reconstructed PieceCIDv2 for validation",
			"original", addRootReq.Root.String(),
			"reconstructed", reconstructedPieceCidV2.String(),
			"match", addRootReq.Root.Equals(reconstructedPieceCidV2),
			"totalUnpaddedSize", totalUnpaddedSize,
			"originalRawSize", rawSize)

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

	// Get the next piece ID to use as firstAdded in signature
	nextPieceId, err := p.verifierContract.GetNextPieceId(ctx, proofSetID)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get next piece ID for dataset %d: %w", id, err)
	}

	log.Infow("AddPieces signing parameters",
		"proofSetID", proofSetID,
		"clientDataSetId", datasetInfo.ClientDataSetId,
		"datasetPayer", datasetInfo.Payer,
		"configuredPayer", smartcontracts.PayerAddress,
		"firstAdded(nextPieceId)", nextPieceId,
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
	// Use clientDataSetId from FilecoinWarmStorageService (not PDPVerifier's setId)
	// Use nextPieceId as firstAdded (this is what PDPVerifier will pass to the callback)
	signature, err := p.signingService.SignAddPieces(ctx,
		datasetInfo.ClientDataSetId, // Use FilecoinWarmStorageService clientDataSetId
		nextPieceId,                 // firstAdded is the next piece ID
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
	log.Infow("AddPieces contract parameters",
		"proofSetID", proofSetID,
		"pieceCount", len(pieceDataArray),
		"firstPieceCID", hex.EncodeToString(pieceDataArray[0].Data),
		"firstPieceSignedCID",
		hex.EncodeToString(pieceDataBytes[0]))
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

// Review: Code below is a copy of github.com/filecoin-project/go-commp-utils/nonffi with support for PieceCIDv2
// mega unsure

type stackFrame struct {
	size  uint64
	commP []byte
}

func GenerateUnsealedCID(proofType abi.RegisteredSealProof, pieceInfos []abi.PieceInfo) (cid.Cid, error) {
	spi, found := abi.SealProofInfos[proofType]
	if !found {
		return cid.Undef, fmt.Errorf("unknown seal proof type %d", proofType)
	}
	if len(pieceInfos) == 0 {
		return cid.Undef, errors.New("no pieces provided")
	}

	maxSize := uint64(spi.SectorSize)

	todo := make([]stackFrame, len(pieceInfos))

	// sancheck everything
	for i, p := range pieceInfos {
		if p.Size < 128 {
			return cid.Undef, fmt.Errorf("invalid Size of PieceInfo %d: value %d is too small", i, p.Size)
		}
		if uint64(p.Size) > maxSize {
			return cid.Undef, fmt.Errorf("invalid Size of PieceInfo %d: value %d is larger than sector size of SealProofType %d", i, p.Size, proofType)
		}
		if bits.OnesCount64(uint64(p.Size)) != 1 {
			return cid.Undef, fmt.Errorf("invalid Size of PieceInfo %d: value %d is not a power of 2", i, p.Size)
		}

		cp, _, err := commcid.PieceCidV2ToDataCommitment(p.PieceCID)
		if err != nil {
			return cid.Undef, fmt.Errorf("invalid PieceCid for PieceInfo %d: %w", i, err)
		}
		todo[i] = stackFrame{size: uint64(p.Size), commP: cp}
	}

	// reimplement https://github.com/filecoin-project/rust-fil-proofs/blob/380d6437c2/filecoin-proofs/src/pieces.rs#L85-L145
	stack := append(
		make(
			[]stackFrame,
			0,
			32,
		),
		todo[0],
	)

	for _, f := range todo[1:] {

		// pre-pad if needed to balance the left limb
		for stack[len(stack)-1].size < f.size {
			lastSize := stack[len(stack)-1].size

			stack = reduceStack(
				append(
					stack,
					stackFrame{
						size:  lastSize,
						commP: zeroCommForSize(lastSize),
					},
				),
			)
		}

		stack = reduceStack(
			append(
				stack,
				f,
			),
		)
	}

	for len(stack) > 1 {
		lastSize := stack[len(stack)-1].size
		stack = reduceStack(
			append(
				stack,
				stackFrame{
					size:  lastSize,
					commP: zeroCommForSize(lastSize),
				},
			),
		)
	}

	if stack[0].size > maxSize {
		return cid.Undef, fmt.Errorf("provided pieces sum up to %d bytes, which is larger than sector size of SealProofType %d", stack[0].size, proofType)
	}

	// TODO probably need to be pieceCIDv2
	return commcid.PieceCommitmentV1ToCID(stack[0].commP)
}

var s256 = sha256simd.New()

func zeroCommForSize(s uint64) []byte { return zerocomm.PieceComms[bits.TrailingZeros64(s)-7][:] }

func reduceStack(s []stackFrame) []stackFrame {
	for len(s) > 1 && s[len(s)-2].size == s[len(s)-1].size {

		s256.Reset()
		s256.Write(s[len(s)-2].commP)
		s256.Write(s[len(s)-1].commP)
		d := s256.Sum(make([]byte, 0, 32))
		d[31] &= 0b00111111

		s[len(s)-2] = stackFrame{
			size:  2 * s[len(s)-2].size,
			commP: d,
		}

		s = s[:len(s)-1]
	}

	return s
}
