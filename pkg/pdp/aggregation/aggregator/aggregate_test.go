package aggregator_test

import (
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/filecoin-project/go-data-segment/merkletree"
	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/storacha/go-libstoracha/piece/digest"
	"github.com/storacha/go-libstoracha/piece/piece"
	"github.com/storacha/piri/pkg/pdp/aggregation/aggregator"
	"github.com/stretchr/testify/require"
)

func TestAggregate(t *testing.T) {
	oddsShrink := 0.8
	oddsReduction := 0.8
	maxSize := 28 // 256 mb
	minSize := 16 // 64 kb
	pieceLinks, err := generatePieceLinks(uint8(maxSize), oddsShrink, oddsReduction, uint8(minSize))
	require.NoError(t, err)
	out, err := json.MarshalIndent(pieceLinks, "", "  ")
	require.NoError(t, err)
	t.Log("piece links\n", string(out))
	agg, err := aggregator.NewAggregate(pieceLinks)
	require.NoError(t, err)
	rootNode := (*merkletree.Node)(agg.Root.DataCommitment())
	for _, aggPiece := range agg.Pieces {
		subTree := (*merkletree.Node)(aggPiece.Link.DataCommitment())
		require.NoError(t, aggPiece.InclusionProof.ValidateSubtree(subTree, rootNode))
	}
}

// this generates a random series of pieces decaying in size that should add up to a size between 2^(height-1) and 2^(height)
func generatePieceLinks(height uint8, oddsShrink float64, oddsReduction float64, smallestSize uint8) ([]piece.PieceLink, error) {
	size := 0
	targetSize := 1 << (height - 1)
	currentHeight := height
	var pieceLinks []piece.PieceLink
	for size <= targetSize {
		for {
			if currentHeight <= smallestSize {
				break
			}
			shouldShrink := rand.Float64() < oddsShrink
			if !shouldShrink {
				break
			}
			currentHeight--
			oddsShrink = oddsShrink * oddsReduction
		}
		paddedSize := 1 << currentHeight
		blobSize := paddedSize/2 + rand.Intn((paddedSize/2)+1-paddedSize/128)

		randLimited := io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), int64(blobSize))
		cp := &commp.Calc{}
		_, err := io.Copy(cp, randLimited)
		if err != nil {
			return nil, err
		}
		commP, actualSize, err := cp.Digest()
		if err != nil {
			return nil, err
		}
		if actualSize != uint64(paddedSize) {
			return nil, errors.New("calculated wrong")
		}
		digest, err := digest.FromCommitmentAndSize(commP, uint64(blobSize))
		if err != nil {
			return nil, err
		}
		pieceLinks = append(pieceLinks, piece.FromPieceDigest(digest))
		size += paddedSize
	}
	return pieceLinks, nil
}
