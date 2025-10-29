package aggregate

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/bits"

	"github.com/filecoin-project/go-commp-utils/v2/zerocomm"
	"github.com/filecoin-project/go-data-segment/merkletree"
	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-libstoracha/piece/digest"
	"github.com/storacha/go-libstoracha/piece/piece"
	"github.com/storacha/go-libstoracha/piece/size"
)

var log = logging.Logger("aggregate")

// This code is adapted from
// https://github.com/filecoin-project/go-commp-utils/blob/master/commd.go
// The goal is to produce an aggregate PieceCID as well as inclusion proofs
// for all the sub CIDs
// It's not a FULL PoDSI style piece cause there is no index written
// Moreover, it's required that all constituent pieces are sorted in descending order
// of size (to avoid unneccesary padding)

type stackFrame struct {
	size  uint64
	commP []byte
	left  *stackFrame
	right *stackFrame
}

func (s stackFrame) isLeaf() bool {
	return s.left == nil
}

// NewAggregate generates an aggregate for a list of pieces that combine in size, and are sorted
// largest to smallest. It returns the aggregate piece link and proof trees for all pieces
func NewAggregate(pieceLinks []piece.PieceLink) (Aggregate, error) {

	if len(pieceLinks) == 0 {
		return Aggregate{}, errors.New("no pieces provided")
	}

	todo := make([]stackFrame, len(pieceLinks))

	// sancheck everything
	lastSize := uint64(0)
	for i, p := range pieceLinks {
		if p.PaddedSize() < 128 {
			return Aggregate{}, fmt.Errorf("invalid Size of PieceInfo %d: value %d is too small", i, p.PaddedSize())
		}
		if lastSize > 0 && p.PaddedSize() > lastSize {
			return Aggregate{}, fmt.Errorf("pieces are not sorted correctly largest to smallest")
		}
		todo[i] = stackFrame{size: p.PaddedSize(), commP: p.DataCommitment()}
		lastSize = p.PaddedSize()
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

	aggregatePieces := make([]AggregatePiece, 0, len(pieceLinks))
	pieceIndex := 0
	err := visitLeaves(&stack[0], func(parents []*stackFrame, index uint64, commP []byte) (bool, error) {
		if !bytes.Equal(pieceLinks[pieceIndex].DataCommitment(), commP) {
			return false, ErrIncorrectTree
		}

		aggregatePieces = append(aggregatePieces, AggregatePiece{
			Link: pieceLinks[pieceIndex],
			InclusionProof: merkletree.ProofData{
				Path:  getProof(parents, index),
				Index: index,
			},
		})
		pieceIndex++
		return pieceIndex < len(pieceLinks), nil
	})
	if err != nil {
		return Aggregate{}, err
	}

	// Calculate actual data size (sum of input pieces before tree padding)
	// Per FRC-0069, the CIDv2 should encode the actual data size, not the padded tree size
	// The padding field in the CID will indicate how much zero-padding was added
	actualDataSize := uint64(0)
	for _, p := range pieceLinks {
		actualDataSize += p.PaddedSize()
	}

	// Log the difference between old and new behavior
	paddedTreeSize := stack[0].size
	log.Errorw("NewAggregate size calculation",
		"actual_data_size", actualDataSize,
		"padded_tree_size", paddedTreeSize,
		"difference", paddedTreeSize-actualDataSize,
		"old_max_data_size", size.MaxDataSize(paddedTreeSize),
		"new_max_data_size", size.MaxDataSize(actualDataSize),
		"stack_size", stack[0].size)

	// Use actual data size, not padded tree size
	digest, err := digest.FromCommitmentAndSize(stack[0].commP, size.MaxDataSize(actualDataSize))
	if err != nil {
		return Aggregate{}, fmt.Errorf("error building aggregate link: %w", err)
	}

	aggregateLink := piece.FromPieceDigest(digest)

	return Aggregate{
		Root:   aggregateLink,
		Pieces: aggregatePieces,
	}, nil
}

func zeroCommForSize(s uint64) []byte { return zerocomm.PieceComms[bits.TrailingZeros64(s)-7][:] }

func reduceStack(s []stackFrame) []stackFrame {

	for len(s) > 1 {
		left := s[len(s)-2]
		right := s[len(s)-1]
		if left.size != right.size {
			break
		}
		commP := computeNode((*merkletree.Node)(left.commP), (*merkletree.Node)(right.commP))

		combined := stackFrame{
			size:  2 * left.size,
			commP: commP[:],
			left:  &left,
			right: &right,
		}
		// replace left node
		s[len(s)-2] = combined
		// pop right node
		s = s[:len(s)-1]
	}

	return s
}

func isRightNode(index uint64) bool {
	return index%2 == 1
}

// getProof generates a proof sequence for a node at the given index with
// the associated parent nodes
func getProof(parents []*stackFrame, index uint64) []merkletree.Node {
	proofs := make([]merkletree.Node, 0, len(parents))
	for i := len(parents) - 1; i >= 0; i-- {
		if isRightNode(index) {
			proofs = append(proofs, *(*merkletree.Node)(parents[i].left.commP))
		} else {
			proofs = append(proofs, *(*merkletree.Node)(parents[i].right.commP))
		}
		index = index / 2
	}
	return proofs
}

// visit leaves traverse a piece tree, visiting leaves along with parents
func visitLeaves(root *stackFrame, visitor func(parents []*stackFrame, index uint64, commP []byte) (bool, error)) error {
	parents := make([]*stackFrame, 0, 32)
	index := uint64(0)
	if root.isLeaf() {
		_, err := visitor(parents, index, root.commP)
		return err
	}
	parents = append(parents, root)
	node := root.left
	for len(parents) > 0 {
		if !node.isLeaf() {
			// go down and left
			parents = append(parents, node)
			node = node.left
			index = index * 2
		} else {
			cont, err := visitor(parents, index, node.commP)
			if err != nil {
				return err
			}
			if !cont {
				return nil
			}
			// go up until we're at a left node
			for isRightNode(index) && len(parents) > 0 {
				parents = parents[:len(parents)-1]
				index = index / 2
			}

			// go right
			if len(parents) > 0 {
				index++
				node = parents[len(parents)-1].right
			}
		}
	}
	return nil
}

// computeNode computes a new internal node in a tree, from its left and right children
func computeNode(left *merkletree.Node, right *merkletree.Node) *merkletree.Node {
	sha := sha256.New()
	sha.Write(left[:])
	sha.Write(right[:])
	digest := sha.Sum(nil)

	return truncate((*merkletree.Node)(digest))
}

func truncate(n *merkletree.Node) *merkletree.Node {
	n[256/8-1] &= 0b00111111
	return n
}
