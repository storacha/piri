package aggregate_test

import (
	"bytes"
	"testing"

	commpUtils "github.com/filecoin-project/go-commp-utils/v2"
	commcid "github.com/filecoin-project/go-fil-commcid"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/storacha/go-libstoracha/piece/digest"
	"github.com/storacha/go-libstoracha/piece/piece"
	"github.com/storacha/piri/pkg/pdp/aggregator/aggregate"
	"github.com/stretchr/testify/require"
)

// TestCIDSizeBug demonstrates that NewAggregate produces incorrect CIDs
// because it uses the wrong size when encoding the CID
func TestCIDSizeBug(t *testing.T) {
	t.Log("=== CID SIZE BUG DEMONSTRATION ===")
	t.Log("The bug is in the CID encoding, not the CommP hash itself")
	t.Log("")

	// Create pieces that sum to 896 bytes (needs padding to 1024)
	sizes := []uint64{512, 256, 128}
	actualDataSize := uint64(896)
	paddedTreeSize := uint64(1024)

	t.Logf("Test setup:")
	t.Logf("  Piece sizes: %v", sizes)
	t.Logf("  Actual data size (sum): %d bytes", actualDataSize)
	t.Logf("  Padded tree size: %d bytes", paddedTreeSize)
	t.Logf("  Padding required: %d bytes", paddedTreeSize-actualDataSize)
	t.Log("")

	// Create identical CommP values for both functions
	commPs := [][]byte{
		makeCommPWithSeed(1),
		makeCommPWithSeed(2),
		makeCommPWithSeed(3),
	}

	// Setup for PieceAggregateCommP
	pieceInfos := make([]abi.PieceInfo, 3)
	for i := range sizes {
		pieceCid, err := commcid.PieceCommitmentV1ToCID(commPs[i])
		require.NoError(t, err)
		pieceInfos[i] = abi.PieceInfo{
			Size:     abi.PaddedPieceSize(sizes[i]),
			PieceCID: pieceCid,
		}
	}

	// Setup for NewAggregate
	pieceLinks := make([]piece.PieceLink, 3)
	for i := range sizes {
		d, err := digest.FromCommitmentAndSize(commPs[i], sizes[i]*127/128)
		require.NoError(t, err)
		pieceLinks[i] = piece.FromPieceDigest(d)
	}

	// Call PieceAggregateCommP
	t.Log("--- PieceAggregateCommP (go-commp-utils) ---")
	commPCid, commPSize, err := commpUtils.PieceAggregateCommP(abi.RegisteredSealProof_StackedDrg32GiBV1_1, pieceInfos)
	require.NoError(t, err)

	commPBytes, err := commcid.CIDToPieceCommitmentV1(commPCid)
	require.NoError(t, err)

	t.Logf("  CommP hash: %x", commPBytes)
	t.Logf("  Size returned: %d", commPSize)
	t.Logf("  CID v1: %s", commPCid)
	t.Log("")

	// Call NewAggregate
	t.Log("--- NewAggregate (piri) ---")
	aggResult, err := aggregate.NewAggregate(pieceLinks)
	require.NoError(t, err)

	aggCommP := aggResult.Root.DataCommitment()
	aggLink := aggResult.Root.Link()

	// Convert link to string for comparison
	aggLinkStr := aggLink.String()

	t.Logf("  CommP hash: %x", aggCommP)
	t.Logf("  Link: %s", aggLinkStr)

	// Now let's check what NewAggregate SHOULD produce if it used the correct size
	t.Log("")
	t.Log("--- What NewAggregate SHOULD produce ---")

	// The bug is on line 111 of build.go:
	// digest.FromCommitmentAndSize(stack[0].commP, size.MaxDataSize(stack[0].size))
	// It uses stack[0].size (paddedTreeSize) instead of actualDataSize

	// Create what the correct digest should be
	correctDigest, err := digest.FromCommitmentAndSize(aggCommP, actualDataSize*127/128) // actual unpadded size
	require.NoError(t, err)
	correctLink := piece.FromPieceDigest(correctDigest)
	correctLinkStr := correctLink.Link().String()

	t.Logf("  Correct Link (with actual data size): %s", correctLinkStr)
	t.Log("")

	// Show the comparison
	t.Log("=== COMPARISON ===")
	t.Logf("CommP hashes match: %v", bytes.Equal(aggCommP, commPBytes))
	t.Logf("Links from go-commp-utils: %s", commPCid.String())
	t.Logf("Links from NewAggregate:   %s", aggLinkStr)
	t.Logf("Links should be:           %s", correctLinkStr)

	if aggLinkStr != correctLinkStr {
		t.Log("")
		t.Errorf("❌ BUG CONFIRMED: NewAggregate produces incorrect Link/CID!")
		t.Errorf("   Actual from NewAggregate: %s", aggLinkStr)
		t.Errorf("   Expected (correct size):  %s", correctLinkStr)
		t.Errorf("")
		t.Errorf("   The bug is on line 111 of build.go:")
		t.Errorf("   It uses: size.MaxDataSize(stack[0].size) where stack[0].size = %d", paddedTreeSize)
		t.Errorf("   Should use: size.MaxDataSize(actualDataSize) where actualDataSize = %d", actualDataSize)
		t.Errorf("")
		t.Errorf("   This violates FRC-0069 which requires the CID to encode the actual data size")
	} else {
		t.Log("✓ Links match - the bug may have been fixed")
	}
}

func makeCommPWithSeed(seed byte) []byte {
	commP := make([]byte, 32)
	for i := range commP {
		commP[i] = seed + byte(i)
	}
	commP[31] &= 0b00111111 // Ensure valid CommP format
	return commP
}
