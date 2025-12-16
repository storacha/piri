package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/filecoin-project/go-commp-utils/nonffi"
	commcid "github.com/filecoin-project/go-fil-commcid"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
	"github.com/samber/lo"
	"github.com/storacha/filecoin-services/go/eip712"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/ipld"
	"github.com/storacha/go-ucanto/core/message"
	"github.com/storacha/go-ucanto/core/receipt"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
	"github.com/storacha/piri/pkg/pdp/tasks"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store/acceptancestore"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

// TODO we need to define non-retryable errors for the add root method, like lack of auth, and lack of dataset else this retries ~50 times
// TODO: Enhanced Crash Recovery Using Deterministic Request IDs:
/*

 Current Implementation Gap:
 There's a window between Send() completing and the database transaction
 where a crash would result in a sent transaction with no database record.
 On restart, duplicate detection wouldn't find anything, leading to duplicate submissions.

 Proposed Solution: Deterministic Request IDs

 Generate a deterministic hash of the request BEFORE calling Send:
   requestID := sha256(proofSetID + sorted(rootCIDs))

 Implementation approach:
 1. Before Send (around line 563):
    // Generate deterministic request ID
    sortedRoots := make([]string, len(rootCIDs))
    copy(sortedRoots, rootCIDs)
    sort.Strings(sortedRoots)

    requestID := sha256.Sum256([]byte(
        fmt.Sprintf("%d:%s", proofSetID, strings.Join(sortedRoots, ","))
    ))
    requestIDHex := "0x" + hex.EncodeToString(requestID[:])

 2. Create DB records with requestID as placeholder txHash:
    if err := p.db.Transaction(func(tx *gorm.DB) error {
        // Insert with deterministic ID
        mw := models.MessageWaitsEth{
            SignedTxHash: requestIDHex,
            TxStatus:     "preparing",
        }
        tx.Create(&mw)

        // Create pdp_proofset_root_adds with requestIDHex
        for _, addReq := range request {
            // ... create records using requestIDHex as AddMessageHash
        }
        return nil
    }); err != nil {
        return common.Hash{}, err
    }

 3. NOW safe to Send - even if we crash, records exist:
    txHash, err := p.sender.Send(ctx, p.address, txEth, reason)

 4. Update records with actual txHash:
    db.Model(&models.MessageWaitsEth{}).
        Where("signed_tx_hash = ?", requestIDHex).
        Update("signed_tx_hash", txHash.Hex())
    // Similarly update pdp_proofset_root_adds

 5. Update duplicate detection to check for both:
    WHERE add_message_hash = ? OR add_message_hash = ?
    -- Check for real txHash OR deterministic requestID

 Benefits:
 - Always leaves a trace in the database before Send
 - Deterministic ID is reproducible from same inputs
 - No special handling needed - looks like a normal hash
 - Handles all crash scenarios:
   * Before DB insert: No trace, safe to proceed
   * After DB insert, before Send: requestID exists, detected as duplicate
   * After Send, before update: Real tx on chain, requestID in DB, still detected
   * After update: Normal state with real txHash

 This approach is simpler than:
 - Parsing encoded transaction data from message_sends_eth
 - Modifying Send to work within transactions (deadlock issues)
 - Adding new tables or complex state management
*/

func (p *PDPService) AddRoots(ctx context.Context, id uint64, request []types.RootAdd) (res common.Hash, retErr error) {
	ctx, span := tracer.Start(ctx, "AddRoots", trace.WithAttributes(
		attribute.Int64("dataset.id", int64(id)),
		attribute.Int("roots.count", len(request)),
	))
	log.Infow("adding roots", "id", id, "request", request)
	defer func() {
		if retErr != nil {
			log.Errorw("failed to add roots", "id", id, "request", request, "err", retErr)
			span.RecordError(retErr)
			span.SetStatus(codes.Error, "failed to add roots")
		} else {
			span.SetAttributes(attribute.Stringer("tx", res))
			log.Infow("added roots", "id", id, "request", request, "response", res)
		}
		span.End()
	}()

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

	// Calculate wait duration for transaction confirmations (used for both new and existing transactions)
	waitDuration := (tasks.MinConfidence + 2) * smartcontracts.FilecoinEpoch

	// Check if any of these roots have already been successfully added to prevent duplicate submissions
	// This handles the case where the node crashes after sending the transaction but before
	// recording it in the database, or when roots have already been fully processed
	rootCIDs := make([]string, len(request))
	for i, req := range request {
		rootCIDs[i] = req.Root.String()
	}

	log.Debugw("Checking for duplicate root submissions",
		"proofset_id", id,
		"root_count", len(rootCIDs))

	// First check pdp_proofset_roots for already successfully added roots
	var existingCompletedRoots []struct {
		Root           string
		AddMessageHash string
	}
	if err := p.db.WithContext(ctx).
		Table("pdp_proofset_roots").
		Select("DISTINCT root, add_message_hash").
		Where("proofset_id = ? AND root IN ?", id, rootCIDs).
		Scan(&existingCompletedRoots).Error; err != nil {
		return common.Hash{}, fmt.Errorf("failed to check for existing completed roots: %w", err)
	}

	if len(existingCompletedRoots) > 0 {
		// Roots have already been successfully added
		txHash := existingCompletedRoots[0].AddMessageHash
		log.Infow("Roots already successfully added, skipping submission",
			"proofset_id", id,
			"tx_hash", txHash,
			"completed_roots", len(existingCompletedRoots))
		return common.HexToHash(txHash), nil
	}

	// Then check pdp_proofset_root_adds for pending additions
	var existingPendingRoots []struct {
		Root           string
		AddMessageHash string
	}
	if err := p.db.WithContext(ctx).
		Table("pdp_proofset_root_adds").
		Select("DISTINCT root, add_message_hash").
		Where("proofset_id = ? AND root IN ?", id, rootCIDs).
		Scan(&existingPendingRoots).Error; err != nil {
		return common.Hash{}, fmt.Errorf("failed to check for existing pending roots: %w", err)
	}

	if len(existingPendingRoots) > 0 {
		span.AddEvent("pending roots exist")
		// Roots are currently being processed - wait for the existing transaction
		txHashStr := existingPendingRoots[0].AddMessageHash
		txHash := common.HexToHash(txHashStr)

		log.Infow("Found existing pending transaction for roots, waiting for confirmation",
			"proofset_id", id,
			"tx_hash", txHashStr,
			"pending_roots", len(existingPendingRoots),
			"wait_duration", waitDuration)

		// Wait for the existing transaction to be confirmed
		// If it succeeds, return success. If it fails, WaitForConfirmation will return an error
		// and the Manager's job queue will automatically retry
		if err := p.WaitForConfirmation(ctx, txHash, waitDuration); err != nil {
			log.Errorw("Existing AddRoots transaction failed or timed out",
				"error", err,
				"tx_hash", txHashStr,
				"proofset_id", id)
			return txHash, fmt.Errorf("existing transaction %s failed or timed out: %w", txHashStr, err)
		}

		log.Infow("Existing AddRoots transaction confirmed successfully",
			"tx_hash", txHashStr,
			"proofset_id", id)
		return txHash, nil
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
		// Reset offset for each root so subroots start at 0 for each root
		var totalOffset uint64 = 0
		// Collect pieceInfos for each subroot.
		pieceInfos := make([]abi.PieceInfo, len(addReq.SubRoots))

		for i, subCID := range addReq.SubRoots {
			subInfo, exists := subrootInfoMap[subCID]
			if !exists {
				return common.Hash{}, fmt.Errorf("subroot CID %s not found in subroot info map", subCID)
			}
			subInfo.SubrootOffset = totalOffset
			pieceInfos[i] = subInfo.PieceInfo
			totalOffset += uint64(subInfo.PieceInfo.Size)
		}

		// GenerateUnsealedCID requires v1PieceCID, so transform here
		var v1SubInfos []abi.PieceInfo
		for _, pi := range pieceInfos {
			v1PieceCID, err := asPieceCIDv1(pi.PieceCID.String())
			if err != nil {
				return common.Hash{}, err
			}
			v1SubInfos = append(v1SubInfos, abi.PieceInfo{
				Size:     pi.Size,
				PieceCID: v1PieceCID,
			})
		}

		// Generate the unsealed CID from the collected piece infos.
		proofType := abi.RegisteredSealProof_StackedDrg64GiBV1_1
		generatedCID, err := nonffi.GenerateUnsealedCID(proofType, v1SubInfos)
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
		span.AddEvent("root", trace.WithAttributes(attribute.Stringer("root", addReq.Root)))
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
		maxPieceSizeLog2, err := p.cachedMaxPieceSizeLog2(ctx)
		if err != nil {
			return common.Hash{}, err
		}
		if uint64(height) > maxPieceSizeLog2.Uint64() {
			return common.Hash{}, fmt.Errorf("invalid height: %d", height)
		}

		var totalSize uint64 = 0
		var prevSubrootSize = subrootInfoMap[addRootReq.SubRoots[0]].PieceInfo.Size
		for i, subrootEntry := range addRootReq.SubRoots {
			subrootInfo := subrootInfoMap[subrootEntry]
			if subrootInfo.PieceInfo.Size > prevSubrootSize {
				// implies a bad request
				return common.Hash{}, fmt.Errorf("subroots must be in descending order of size, root %d %s is larger than prev subroot %s", i, subrootEntry, addRootReq.SubRoots[i-1])
			}
			prevSubrootSize = subrootInfo.PieceInfo.Size

			paddedSize := uint64(subrootInfo.PieceInfo.Size)
			totalSize += paddedSize
		}

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

	proofs := make([][]ipld.Link, 0, len(request))
	proofData := make([][]message.AgentMessage, 0, len(request))
	for _, req := range request {
		tasks := make([]ipld.Link, 0, len(req.SubRoots))
		msgs := make([]message.AgentMessage, 0, len(req.SubRoots))
		for _, subroot := range req.SubRoots {
			task, msg, err := getAddPieceProofs(ctx, p.pieceResolver, p.acceptanceStore, p.receiptStore, subroot)
			if err != nil {
				return common.Hash{}, fmt.Errorf("getting proofs to add piece %s: %w", subroot, err)
			}
			tasks = append(tasks, task)
			msgs = append(msgs, msg)
		}
		proofs = append(proofs, tasks)
		proofData = append(proofData, msgs)
	}

	// Request a signature for adding pieces from the signing service.
	// Use clientDataSetId from FilecoinWarmStorageService (not PDPVerifier's setId).
	// Generate a random nonce so it never collides with values stored in clientNonces during createDataSet.
	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		return common.Hash{}, fmt.Errorf("failed to generate nonce: %w", err)
	}
	nonce := new(big.Int).SetBytes(nonceBytes)
	// TODO(ash/forrst): nil is bad mkay, don't do this in the release....
	signature, err := p.signingService.SignAddPieces(ctx,
		p.id,
		datasetInfo.ClientDataSetId, // Use FilecoinWarmStorageService clientDataSetId
		nonce,                       // client-chosen nonce, disjoint from createDataSet clientDataSetId
		pieceDataBytes,
		metadata,
		proofs,
		proofData,
	)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to sign AddPieces: %w", err)
	}

	// Encode the extraData with signature and metadata
	extraDataBytes, err := p.edc.EncodeAddPiecesExtraData(nonce, signature, metadata)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to encode extraData: %w", err)
	}

	// listener must be empty address for datasets that already exist, thus 3rd argument.
	data, err := abiData.Pack("addPieces", proofSetID, common.Address{}, pieceDataArray, extraDataBytes)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to pack addRoots: %w", err)
	}

	// Prepare the transaction (nonce will be set to 0, SenderETH will assign it)
	txEth := ethtypes.NewTransaction(
		0,
		p.cfg.Contracts.Verifier,
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
	span.AddEvent("transaction sent")

	// Step 9: Insert into message_waits_eth and pdp_proofset_root_adds
	if err := p.db.Transaction(func(tx *gorm.DB) error {
		// Insert into message_waits_eth
		mw := models.MessageWaitsEth{
			SignedTxHash: txHash.Hex(),
			TxStatus:     "pending",
		}
		if err := tx.WithContext(ctx).Create(&mw).Error; err != nil {
			return err
		}

		// Update proof set for initialization upon first add TODO this is idempotent query, but can be avoided if we are sure the proofset is already ready we should also wait to say its ready until the root has landed on chain
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

	// Step 10: Wait for the transaction to be confirmed on chain
	// This prevents the race condition where multiple parallel AddRoots calls
	// all read the same nextPieceId but only one can succeed
	log.Infow("waiting for AddRoots transaction confirmation", "txHash", txHash.Hex(), "proofSetID", proofSetID, "waitDuration", waitDuration)
	if err := p.WaitForConfirmation(ctx, txHash, waitDuration); err != nil {
		log.Errorw("AddRoots transaction failed or timed out", "error", err, "txHash", txHash.Hex(), "proofSetID", proofSetID)
		return txHash, fmt.Errorf("transaction %s failed or timed out: %w", txHash.Hex(), err)
	}

	log.Infow("AddRoots transaction confirmed successfully", "txHash", txHash.Hex(), "proofSetID", proofSetID)
	return txHash, nil
}

// just the bit of the piece resolver API that we need
type blobResolvable interface {
	ResolveToBlob(ctx context.Context, piece multihash.Multihash) (multihash.Multihash, bool, error)
}

func getAddPieceProofs(
	ctx context.Context,
	resolver blobResolvable,
	accStore acceptancestore.AcceptanceStore,
	rcptStore receiptstore.ReceiptStore,
	piece cid.Cid,
) (ipld.Link, message.AgentMessage, error) {
	blob, ok, err := resolver.ResolveToBlob(ctx, piece.Hash())
	if err != nil {
		return nil, nil, fmt.Errorf("resolving piece to blob hash: %w", err)
	}
	if !ok {
		return nil, nil, fmt.Errorf("missing piece to blob mapping: %s", piece)
	}

	// We can accept the same blob in multiple _spaces_, but we only add a root
	// for the blob to PDP once. So it doesn't really matter which acceptance
	// record we retrieve here, but there will only be one anyway, since this will
	// be the first (and only) time this blob is added to PDP.
	accs, err := accStore.List(ctx, blob)
	if err != nil {
		return nil, nil, fmt.Errorf("listing acceptances: %w", err)
	}
	if len(accs) == 0 {
		return nil, nil, fmt.Errorf("missing acceptance: %w", err)
	}
	acc := accs[0]
	if acc.PDPAccept == nil {
		return nil, nil, errors.New("missing PDP accept promise")
	}

	// The `blob/accept` invocation and receipt proves the node was asked to store
	// the data, or more accurately, it was asked _and_ it confirmed it received
	// the data.
	blobAccRcpt, err := rcptStore.GetByRan(ctx, acc.Cause)
	if err != nil {
		return nil, nil, fmt.Errorf("getting blob/accept receipt: %w", err)
	}
	// expect invocation to be attached to receipt
	blobAccInv, ok := blobAccRcpt.Ran().Invocation()
	if !ok {
		return nil, nil, fmt.Errorf("missing blob/accept invocation: %w", err)
	}

	// The `pdp/accept` invocation and receipt proves the node calculated a CommP
	// for the blob and aggregated it into an aggregate piece. Note: It doesn't
	// prove the equality relationship between the blob hash and the piece CID.
	pdpAccRcpt, err := rcptStore.GetByRan(ctx, acc.PDPAccept.UcanAwait.Link)
	if err != nil {
		return nil, nil, fmt.Errorf("getting pdp/accept receipt: %w", err)
	}
	pdpAccInv, ok := pdpAccRcpt.Ran().Invocation()
	if !ok {
		return nil, nil, fmt.Errorf("missing pdp/accept invocation: %w", err)
	}

	// The blob/accept receipt contains the link to the pdp/accept invocation in
	// effects. Here we combine the blocks of these two related receipts (and
	// invocations) in an agent message.
	msg, err := message.Build(
		[]invocation.Invocation{blobAccInv, pdpAccInv},
		[]receipt.AnyReceipt{blobAccRcpt, pdpAccRcpt},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("building agent message: %w", err)
	}
	return acc.Cause, msg, nil
}
