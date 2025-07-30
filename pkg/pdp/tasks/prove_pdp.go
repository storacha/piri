package tasks

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"math/bits"
	"sort"
	"sync/atomic"

	"github.com/minio/sha256-simd"
	"github.com/samber/lo"
	"golang.org/x/crypto/sha3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/filecoin-project/go-commp-utils/zerocomm"
	commcid "github.com/filecoin-project/go-fil-commcid"
	"github.com/filecoin-project/go-state-types/abi"
	chaintypes "github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/storage/pipeline/lib/nullreader"

	"github.com/ipfs/go-cid"

	pool "github.com/libp2p/go-buffer-pool"

	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/ethereum"
	"github.com/storacha/piri/pkg/pdp/promise"
	"github.com/storacha/piri/pkg/pdp/proof"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service/contract"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/store/blobstore"
)

var _ scheduler.TaskInterface = &ProveTask{}

const LeafSize = proof.NODE_SIZE

type ProveTask struct {
	db             *gorm.DB
	ethClient      bind.ContractBackend
	contractClient contract.PDP
	sender         ethereum.Sender
	bs             blobstore.Blobstore
	api            ChainAPI

	head atomic.Pointer[chaintypes.TipSet]

	addFunc promise.Promise[scheduler.AddTaskFunc]
}

func NewProveTask(
	chainSched *chainsched.Scheduler,
	db *gorm.DB,
	ethClient bind.ContractBackend,
	contractClient contract.PDP,
	api ChainAPI,
	sender ethereum.Sender,
	bs blobstore.Blobstore,
) (*ProveTask, error) {
	pt := &ProveTask{
		db:             db,
		ethClient:      ethClient,
		contractClient: contractClient,
		sender:         sender,
		api:            api,
		bs:             bs,
	}

	// ProveTasks are created on pdp_proof_sets entries where
	// challenge_request_msg_hash is not null (=not yet landed)

	err := chainSched.AddHandler(func(ctx context.Context, revert, apply *chaintypes.TipSet) error {
		if apply == nil {
			return nil
		}

		pt.head.Store(apply)

		for {
			more := false

			pt.addFunc.Val(ctx)(func(id scheduler.TaskID, tx *gorm.DB) (shouldCommit bool, seriousError error) {
				// Select proof sets ready for proving
				var proofSets []struct {
					ID int64
				}
				if err := tx.Table("pdp_proof_sets as p").
					Select("p.id").
					Joins("INNER JOIN message_waits_eth as mw ON mw.signed_tx_hash = p.challenge_request_msg_hash").
					Where("p.challenge_request_msg_hash IS NOT NULL").
					Where("mw.tx_success = ?", true).
					Where("p.prove_at_epoch < ?", apply.Height()).
					Limit(2).
					Scan(&proofSets).Error; err != nil {
					return false, fmt.Errorf("failed to select proof sets: %w", err)
				}

				if len(proofSets) == 0 {
					// No proof sets to process
					return false, nil
				}

				// Determine if there might be more proof sets to process
				more = len(proofSets) == 2

				// Process the first proof set
				todo := proofSets[0]

				// Insert a new task into pdp_prove_tasks
				result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.PDPProveTask{
					ProofsetID: todo.ID,
					TaskID:     int64(id),
				})
				if result.Error != nil {
					return false, fmt.Errorf("failed to insert into pdp_prove_tasks: %w", result.Error)
				}
				if result.RowsAffected == 0 {
					return false, nil
				}

				// Update pdp_proof_sets to set next_challenge_possible = FALSE
				result = tx.Model(&models.PDPProofSet{}).
					Where("id = ? AND challenge_request_msg_hash IS NOT NULL", todo.ID).
					Update("challenge_request_msg_hash", nil)
				if result.Error != nil {
					return false, fmt.Errorf("failed to update pdp_proof_sets: %w", result.Error)
				}
				if result.RowsAffected == 0 {
					more = false
					return false, nil
				}

				return true, nil
			})

			if !more {
				break
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to register pdp ProveTask: %w", err)
	}

	return pt, nil
}

func (p *ProveTask) Do(taskID scheduler.TaskID) (done bool, err error) {
	ctx := context.Background()

	// Retrieve proof set and challenge epoch for the task
	var proveTask models.PDPProveTask
	if err := p.db.Where("task_id = ?", taskID).First(&proveTask).Error; err != nil {
		return false, fmt.Errorf("failed to get task details: %w", err)
	}
	proofSetID := proveTask.ProofsetID

	pdpContracts := contract.Addresses()
	pdpVerifierAddress := pdpContracts.PDPVerifier

	pdpVerifier, err := p.contractClient.NewPDPVerifier(pdpVerifierAddress, p.ethClient)
	if err != nil {
		return false, fmt.Errorf("failed to instantiate PDPVerifier contract at %s: %w", pdpVerifierAddress.Hex(), err)
	}

	callOpts := &bind.CallOpts{
		Context: ctx,
	}

	// Proof parameters
	challengeEpoch, err := pdpVerifier.GetNextChallengeEpoch(callOpts, big.NewInt(proofSetID))
	if err != nil {
		return false, fmt.Errorf("failed to get next challenge epoch: %w", err)
	}

	seed, err := p.api.StateGetRandomnessDigestFromBeacon(ctx, abi.ChainEpoch(challengeEpoch.Int64()), chaintypes.EmptyTSK)
	if err != nil {
		return false, fmt.Errorf("failed to get chain randomness from beacon for pdp prove: %w", err)
	}

	proofs, err := p.GenerateProofs(ctx, pdpVerifier, proofSetID, seed, contract.NumChallenges)
	if err != nil {
		return false, fmt.Errorf("failed to generate proofs: %w", err)
	}

	abiData, err := contract.PDPVerifierMetaData()
	if err != nil {
		return false, fmt.Errorf("failed to get PDPVerifier ABI: %w", err)
	}

	data, err := abiData.Pack("provePossession", big.NewInt(proofSetID), proofs)
	if err != nil {
		return false, fmt.Errorf("failed to pack data: %w", err)
	}

	// [ ["0x559e581f022bb4e4ec6e719e563bf0e026ad6de42e56c18714a2c692b1b88d7e", ["0x559e581f022bb4e4ec6e719e563bf0e026ad6de42e56c18714a2c692b1b88d7e"]] ]

	/* {
		// format proofs for logging
		var proofStr string = "[ [\"0x"
		proofStr += hex.EncodeToString(proofs[0].Leaf[:])
		proofStr += "\", ["
		for i, proof := range proofs[0].Proof {
			if i > 0 {
				proofStr += ", "
			}
			proofStr += "\"0x"
			proofStr += hex.EncodeToString(proof[:])
			proofStr += "\""
		}

		proofStr += "] ] ]"

		log.Infof("PDP Prove Task: proofSetID: %d, taskID: %d, proofs: %s", proofSetID, taskID, proofStr)
	} */

	// If gas used is 0 fee is maximized
	gasFee := big.NewInt(0)
	proofFee, err := pdpVerifier.CalculateProofFee(callOpts, big.NewInt(proofSetID), gasFee)
	if err != nil {
		return false, fmt.Errorf("failed to calculate proof fee: %w", err)
	}

	// Add 2x buffer for certainty
	proofFee = new(big.Int).Mul(proofFee, big.NewInt(3))

	// TODO need to validate this is okay, previously in curio this was pulled form the DB, though I think
	// this is the same address
	fromAddress, _, err := pdpVerifier.GetProofSetOwner(nil, big.NewInt(proofSetID))
	if err != nil {
		return false, fmt.Errorf("failed to get default sender address: %w", err)
	}

	// Prepare the transaction (nonce will be set to 0, SenderETH will assign it)
	txEth := types.NewTransaction(
		0,
		pdpVerifierAddress,
		proofFee,
		0,
		nil,
		data,
	)

	log.Infow("PDP Prove Task",
		"proofSetID", proofSetID,
		"taskID", taskID,
		"gasFeeEstimate", gasFee,
		"txEth", txEth,
	)

	reason := "pdp-prove"
	txHash, err := p.sender.Send(ctx, fromAddress, txEth, reason)
	if err != nil {
		return false, fmt.Errorf("failed to send transaction: %w", err)
	}

	// Remove the roots previously scheduled for deletion
	err = p.cleanupDeletedRoots(ctx, proofSetID, pdpVerifier)
	if err != nil {
		return false, fmt.Errorf("failed to cleanup deleted roots: %w", err)
	}

	log.Infow("PDP Prove Task: transaction sent", "txHash", txHash, "proofSetID", proofSetID, "taskID", taskID)

	// Task completed successfully
	return true, nil
}

func (p *ProveTask) GenerateProofs(ctx context.Context, pdpService contract.PDPVerifier, proofSetID int64, seed abi.Randomness, numChallenges int) ([]contract.PDPVerifierProof, error) {
	proofs := make([]contract.PDPVerifierProof, numChallenges)

	callOpts := &bind.CallOpts{
		Context: ctx,
	}

	totalLeafCount, err := pdpService.GetChallengeRange(callOpts, big.NewInt(proofSetID))
	if err != nil {
		return nil, fmt.Errorf("failed to get proof set leaf count: %w", err)
	}
	totalLeaves := totalLeafCount.Uint64()

	challenges := lo.Times(numChallenges, func(i int) int64 {
		return generateChallengeIndex(seed, proofSetID, i, totalLeaves)
	})

	rootId, err := pdpService.FindRootIds(callOpts, big.NewInt(proofSetID), lo.Map(challenges, func(i int64, _ int) *big.Int { return big.NewInt(i) }))
	if err != nil {
		return nil, fmt.Errorf("failed to find root IDs: %w", err)
	}

	for i := 0; i < numChallenges; i++ {
		root := rootId[i]

		proof, err := p.proveRoot(ctx, proofSetID, root.RootId.Int64(), root.Offset.Int64())
		if err != nil {
			return nil, fmt.Errorf("failed to prove root %d (%d, %d, %d): %w", i, proofSetID, root.RootId.Int64(), root.Offset.Int64(), err)
		}

		proofs[i] = proof
	}

	return proofs, nil
}

func generateChallengeIndex(seed abi.Randomness, proofSetID int64, proofIndex int, totalLeaves uint64) int64 {
	// Create a buffer to hold the concatenated data (96 bytes: 32 bytes * 3)
	data := make([]byte, 0, 96)

	// Seed is a 32-byte big-endian representation

	data = append(data, seed...)

	// Convert proofSetID to 32-byte big-endian representation
	proofSetIDBigInt := big.NewInt(proofSetID)
	proofSetIDBytes := padTo32Bytes(proofSetIDBigInt.Bytes())
	data = append(data, proofSetIDBytes...)

	// Convert proofIndex to 8-byte big-endian representation
	proofIndexBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(proofIndexBytes, uint64(proofIndex))
	data = append(data, proofIndexBytes...)

	// Compute the Keccak-256 hash
	hash := sha3.NewLegacyKeccak256()
	hash.Write(data)
	hashBytes := hash.Sum(nil)

	// Convert hash to big.Int
	hashInt := new(big.Int).SetBytes(hashBytes)

	// Compute challenge index
	totalLeavesBigInt := new(big.Int).SetUint64(totalLeaves)
	challengeIndex := new(big.Int).Mod(hashInt, totalLeavesBigInt)

	// Log for debugging
	log.Debugw("generateChallengeIndex",
		"seed", seed,
		"proofSetID", proofSetID,
		"proofIndex", proofIndex,
		"totalLeaves", totalLeaves,
		"data", hex.EncodeToString(data),
		"hash", hex.EncodeToString(hashBytes),
		"hashInt", hashInt,
		"totalLeavesBigInt", totalLeavesBigInt,
		"challengeIndex", challengeIndex,
	)

	return challengeIndex.Int64()
}

// padTo32Bytes pads the input byte slice to 32 bytes with leading zeros
func padTo32Bytes(b []byte) []byte {
	padded := make([]byte, 32)
	copy(padded[32-len(b):], b)
	return padded
}

func (p *ProveTask) genSubrootMemtree(ctx context.Context, subrootCid string, subrootSize abi.PaddedPieceSize) ([]byte, error) {
	subrootCidObj, err := cid.Parse(subrootCid)
	if err != nil {
		return nil, fmt.Errorf("failed to parse subroot CID: %w", err)
	}

	if subrootSize > proof.MaxMemtreeSize {
		return nil, fmt.Errorf("subroot size exceeds maximum: %d", subrootSize)
	}

	// TODO everything below here is probably wrong with respect to size's
	sr, err := p.bs.Get(ctx, subrootCidObj.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get subroot reader: %w", err)
	}

	var r io.Reader = sr.Body()

	if sr.Size() > int64(subrootSize) {
		return nil, fmt.Errorf("subroot size mismatch: %d > %d", sr.Size(), subrootSize)
	} else if sr.Size() < int64(subrootSize) {
		// pad with zeros
		r = io.MultiReader(r, nullreader.NewNullReader(abi.UnpaddedPieceSize(int64(subrootSize)-sr.Size())))
	}

	// TODO unsure how closing the object from a blobstore works.
	// defer sr.Body().Close()

	return proof.BuildSha254Memtree(r, subrootSize.Unpadded())
}

func (p *ProveTask) proveRoot(ctx context.Context, proofSetID int64, rootId int64, challengedLeaf int64) (contract.PDPVerifierProof, error) {
	const arity = 2

	rootChallengeOffset := challengedLeaf * LeafSize
	// Use a local type to hold the selected columns
	type subrootMeta struct {
		Root          string `gorm:"column:root"`
		Subroot       string `gorm:"column:subroot"`
		SubrootOffset int64  `gorm:"column:subroot_offset"`
		SubrootSize   int64  `gorm:"column:subroot_size"`
	}

	var subroots []subrootMeta
	if err := p.db.Table("pdp_proofset_roots").
		Select("root, subroot, subroot_offset, subroot_size").
		Where("proofset_id = ? AND root_id = ?", proofSetID, rootId).
		Order("subroot_offset ASC").
		Scan(&subroots).Error; err != nil {
		return contract.PDPVerifierProof{}, fmt.Errorf("failed to get root and subroot: %w", err)
	}

	// find first subroot with subroot_offset >= rootChallengeOffset
	challSubRoot, challSubrootIdx, ok := lo.FindLastIndexOf(subroots, func(subroot subrootMeta) bool {
		return subroot.SubrootOffset < rootChallengeOffset
	})
	if !ok {
		return contract.PDPVerifierProof{}, fmt.Errorf("no subroot found")
	}

	// build subroot memtree
	memtree, err := p.genSubrootMemtree(ctx, challSubRoot.Subroot, abi.PaddedPieceSize(challSubRoot.SubrootSize))
	if err != nil {
		return contract.PDPVerifierProof{}, fmt.Errorf("failed to generate subroot memtree: %w", err)
	}

	subrootChallengedLeaf := challengedLeaf - (challSubRoot.SubrootOffset / LeafSize)
	log.Debugw("subrootChallengedLeaf", "subrootChallengedLeaf", subrootChallengedLeaf, "challengedLeaf", challengedLeaf, "subrootOffsetLs", challSubRoot.SubrootOffset/LeafSize)

	/*
		type RawMerkleProof struct {
			Leaf  [32]byte
			Proof [][32]byte
			Root  [32]byte
		}
	*/
	subrootProof, err := proof.MemtreeProof(memtree, subrootChallengedLeaf)
	pool.Put(memtree)
	if err != nil {
		return contract.PDPVerifierProof{}, fmt.Errorf("failed to generate subroot proof: %w", err)
	}
	log.Debugw("subrootProof", "subrootProof", subrootProof)

	// build partial top-tree
	type treeElem struct {
		Level int // 1 == leaf, NODE_SIZE
		Hash  [LeafSize]byte
	}
	type elemIndex struct {
		Level      int
		ElemOffset int64 // offset in terms of nodes at the current level
	}

	partialTree := map[elemIndex]treeElem{}
	var subrootsSize abi.PaddedPieceSize

	// 1. prefill the partial tree
	for _, subroot := range subroots {
		subrootsSize += abi.PaddedPieceSize(subroot.SubrootSize)

		unsCid, err := cid.Parse(subroot.Subroot)
		if err != nil {
			return contract.PDPVerifierProof{}, fmt.Errorf("failed to parse subroot CID: %w", err)
		}

		commp, err := commcid.CIDToPieceCommitmentV1(unsCid)
		if err != nil {
			return contract.PDPVerifierProof{}, fmt.Errorf("failed to convert CID to piece commitment: %w", err)
		}

		var comm [LeafSize]byte
		copy(comm[:], commp)

		level := proof.NodeLevel(subroot.SubrootSize/LeafSize, arity)
		offset := (subroot.SubrootOffset / LeafSize) >> uint(level-1)
		partialTree[elemIndex{Level: level, ElemOffset: offset}] = treeElem{
			Level: level,
			Hash:  comm,
		}
	}

	rootSize := nextPowerOfTwo(subrootsSize)
	rootLevel := proof.NodeLevel(int64(rootSize/LeafSize), arity)

	// 2. build the partial tree
	// we do the build from the right side of the tree - elements are sorted by size, so only elements on the right side can have missing siblings

	isRight := func(offset int64) bool {
		return offset&1 == 1
	}

	for i := len(subroots) - 1; i >= 0; i-- {
		subroot := subroots[i]
		level := proof.NodeLevel(subroot.SubrootSize/LeafSize, arity)
		offset := (subroot.SubrootOffset / LeafSize) >> uint(level-1)
		firstSubroot := i == 0

		curElem := partialTree[elemIndex{Level: level, ElemOffset: offset}]

		log.Debugw("processing partialtree subroot", "curElem", curElem, "level", level, "offset", offset, "subroot", subroot.SubrootOffset, "subrootSz", subroot.SubrootSize)

		for !isRight(offset) {
			// find the rightSibling
			siblingIndex := elemIndex{Level: level, ElemOffset: offset + 1}
			rightSibling, ok := partialTree[siblingIndex]
			if !ok {
				// if we're processing the first subroot branch, AND we've ran out of right siblings, we're done
				if firstSubroot {
					break
				}

				// create a zero rightSibling
				rightSibling = treeElem{
					Level: level,
					Hash:  zerocomm.PieceComms[level-zerocomm.Skip-1],
				}
				log.Debugw("rightSibling zero", "rightSibling", rightSibling, "siblingIndex", siblingIndex, "level", level, "offset", offset)
				partialTree[siblingIndex] = rightSibling
			}

			// compute the parent
			parent := proof.ComputeBinShaParent(curElem.Hash, rightSibling.Hash)
			parentLevel := level + 1
			parentOffset := offset / arity

			partialTree[elemIndex{Level: parentLevel, ElemOffset: parentOffset}] = treeElem{
				Level: parentLevel,
				Hash:  parent,
			}

			// move to the parent
			level = parentLevel
			offset = parentOffset
			curElem = partialTree[elemIndex{Level: level, ElemOffset: offset}]
		}
	}

	{
		var partialTreeList []elemIndex
		for k := range partialTree {
			partialTreeList = append(partialTreeList, k)
		}
		sort.Slice(partialTreeList, func(i, j int) bool {
			if partialTreeList[i].Level != partialTreeList[j].Level {
				return partialTreeList[i].Level < partialTreeList[j].Level
			}
			return partialTreeList[i].ElemOffset < partialTreeList[j].ElemOffset
		})

	}

	challLevel := proof.NodeLevel(challSubRoot.SubrootSize/LeafSize, arity)
	challOffset := (challSubRoot.SubrootOffset / LeafSize) >> uint(challLevel-1)

	log.Debugw("challSubRoot", "challSubRoot", challSubrootIdx, "challLevel", challLevel, "challOffset", challOffset)

	challSubtreeLeaf := partialTree[elemIndex{Level: challLevel, ElemOffset: challOffset}]
	if challSubtreeLeaf.Hash != subrootProof.Root {
		return contract.PDPVerifierProof{}, fmt.Errorf("subtree root doesn't match partial tree leaf, %x != %x", challSubtreeLeaf.Hash, subrootProof.Root)
	}

	var out contract.PDPVerifierProof
	copy(out.Leaf[:], subrootProof.Leaf[:])
	out.Proof = append(out.Proof, subrootProof.Proof...)

	currentLevel := challLevel
	currentOffset := challOffset

	for currentLevel < rootLevel {
		siblingOffset := currentOffset ^ 1

		// Retrieve sibling hash from partialTree or use zero hash
		siblingIndex := elemIndex{Level: currentLevel, ElemOffset: siblingOffset}
		index := elemIndex{Level: currentLevel, ElemOffset: currentOffset}
		siblingElem, ok := partialTree[siblingIndex]
		if !ok {
			return contract.PDPVerifierProof{}, fmt.Errorf("missing sibling at level %d, offset %d", currentLevel, siblingOffset)
		}
		elem, ok := partialTree[index]
		if !ok {
			return contract.PDPVerifierProof{}, fmt.Errorf("missing element at level %d, offset %d", currentLevel, currentOffset)
		}
		if currentOffset < siblingOffset { // left
			log.Debugw("Proof", "position", index, "left-c", hex.EncodeToString(elem.Hash[:]), "right-s", hex.EncodeToString(siblingElem.Hash[:]), "out", hex.EncodeToString(shabytes(append(elem.Hash[:], siblingElem.Hash[:]...))[:]))
		} else { // right
			log.Debugw("Proof", "position", index, "left-s", hex.EncodeToString(siblingElem.Hash[:]), "right-c", hex.EncodeToString(elem.Hash[:]), "out", hex.EncodeToString(shabytes(append(siblingElem.Hash[:], elem.Hash[:]...))[:]))
		}

		// Append the sibling's hash to the proof
		out.Proof = append(out.Proof, siblingElem.Hash)

		// Move up to the parent node
		currentOffset = currentOffset / arity
		currentLevel++
	}

	log.Debugw("proof complete", "proof", out)

	rootCid, err := cid.Parse(subroots[0].Root)
	if err != nil {
		return contract.PDPVerifierProof{}, fmt.Errorf("failed to parse root CID: %w", err)
	}
	commRoot, err := commcid.CIDToPieceCommitmentV1(rootCid)
	if err != nil {
		return contract.PDPVerifierProof{}, fmt.Errorf("failed to convert CID to piece commitment: %w", err)
	}
	var cr [LeafSize]byte
	copy(cr[:], commRoot)

	if !Verify(out, cr, uint64(challengedLeaf)) {
		return contract.PDPVerifierProof{}, fmt.Errorf("proof verification failed")
	}

	// Return the completed proof
	return out, nil
}

func (p *ProveTask) cleanupDeletedRoots(ctx context.Context, proofSetID int64, pdpVerifier contract.PDPVerifier) error {
	removals, err := pdpVerifier.GetScheduledRemovals(nil, big.NewInt(proofSetID))
	if err != nil {
		return fmt.Errorf("failed to get scheduled removals: %w", err)
	}

	// Execute cleanup in a transaction
	err = p.db.Transaction(func(tx *gorm.DB) error {
		for _, removeID := range removals {
			var proofsetRoot models.PDPProofsetRoot
			if err := tx.Where("proofset_id = ? AND root_id = ?", proofSetID, removeID.Int64()).First(&proofsetRoot).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					// Root already deleted, skip
					continue
				}
				return fmt.Errorf("failed to get piece ref for root %d: %w", removeID, err)
			}

			if err := tx.Delete(&models.ParkedPieceRef{}, proofsetRoot.PDPPieceRefID).Error; err != nil {
				return fmt.Errorf("failed to delete parked piece ref %d: %w", proofsetRoot.PDPPieceRefID, err)
			}

			if err := tx.Where("proofset_id = ? AND root_id = ?", proofSetID, removeID).Delete(&models.PDPProofsetRoot{}).Error; err != nil {
				return fmt.Errorf("failed to delete root %d: %w", removeID, err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to cleanup deleted roots: %w", err)
	}

	return nil
}

func (p *ProveTask) CanAccept(ids []scheduler.TaskID, engine *scheduler.TaskEngine) (*scheduler.TaskID, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	id := ids[0]
	return &id, nil
}

func (p *ProveTask) TypeDetails() scheduler.TaskTypeDetails {
	return scheduler.TaskTypeDetails{
		Name:        "PDPProve",
		MaxFailures: 5,
	}
}

func (p *ProveTask) Adder(taskFunc scheduler.AddTaskFunc) {
	p.addFunc.Set(taskFunc)
}

func nextPowerOfTwo(n abi.PaddedPieceSize) abi.PaddedPieceSize {
	lz := bits.LeadingZeros64(uint64(n - 1))
	return 1 << (64 - lz)
}

func Verify(proof contract.PDPVerifierProof, root [32]byte, position uint64) bool {
	computedHash := proof.Leaf

	for i := 0; i < len(proof.Proof); i++ {
		sibling := proof.Proof[i]

		if position%2 == 0 {
			log.Debugw("Verify", "position", position, "left-c", hex.EncodeToString(computedHash[:]), "right-s", hex.EncodeToString(sibling[:]), "out", hex.EncodeToString(shabytes(append(computedHash[:], sibling[:]...))[:]))
			// If position is even, current node is on the left
			computedHash = sha256.Sum256(append(computedHash[:], sibling[:]...))
		} else {
			log.Debugw("Verify", "position", position, "left-s", hex.EncodeToString(sibling[:]), "right-c", hex.EncodeToString(computedHash[:]), "out", hex.EncodeToString(shabytes(append(sibling[:], computedHash[:]...))[:]))
			// If position is odd, current node is on the right
			computedHash = sha256.Sum256(append(sibling[:], computedHash[:]...))
		}
		computedHash[31] &= 0x3F // set top bits to 00

		// Move up to the parent node
		position /= 2
	}

	// Compare the reconstructed root with the expected root
	return computedHash == root
}

func shabytes(in []byte) []byte {
	out := sha256.Sum256(in)
	return out[:]
}
