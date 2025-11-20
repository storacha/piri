package aggregator_test

import (
	"testing"

	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/piri/pkg/pdp/aggregation/aggregator"
	"github.com/storacha/piri/pkg/pdp/aggregation/types"
	"github.com/stretchr/testify/require"

	"github.com/storacha/go-libstoracha/piece/piece"
)

// Human-friendly byte sizes
const (
	MB = 1 << 20
)

func TestAggregatePieces(t *testing.T) {
	tests := []struct {
		name               string
		pieceSizes         []int64 // unpadded sizes
		expectedBufferSize uint64
		expectedAggCount   int
	}{
		//
		// generally happy path
		//
		{
			name:               "Single piece <128MB remains buffered (no aggregate)",
			pieceSizes:         []int64{32 * MB},
			expectedBufferSize: 64 * MB, // after rounding
			expectedAggCount:   0,
		},
		{
			name: "Two pieces together exceed 128MB => 1 aggregate, buffer cleared",
			pieceSizes: []int64{
				32 * MB, // ~64MB padded
				32 * MB, // ~64MB padded => total ~128MB => triggers an aggregate
			},
			expectedBufferSize: 0,
			expectedAggCount:   1,
		},
		{
			name: "Three pieces => first two trigger an aggregate, third remains in buffer",
			pieceSizes: []int64{
				32 * MB, // ~64MB padded
				32 * MB, // ~64MB padded => crossing 128MB threshold
				32 * MB, // this stays in the buffer after the first aggregate is formed
			},
			// By the time the 2nd piece is processed, we cross 128MB => an aggregate is created.
			// The 3rd piece goes into a new empty buffer => 64MB remains there.
			expectedBufferSize: 64 * MB,
			expectedAggCount:   1,
		},
		{
			name: "Four pieces => triggers two aggregates, ending with empty buffer",
			pieceSizes: []int64{
				32 * MB, // ~64MB padded
				32 * MB, // ~64MB padded => triggers first aggregate
				32 * MB, // ~64MB padded
				32 * MB, // ~64MB padded => triggers second aggregate
			},
			expectedBufferSize: 0,
			expectedAggCount:   2,
		},
		{
			name: "Two large pieces >128MB each => immediate aggregate per piece",
			pieceSizes: []int64{
				130 * MB, // > 128MB => triggers immediate
				200 * MB, // also > 128MB => triggers immediate
			},
			expectedBufferSize: 0,
			expectedAggCount:   2,
		},
		//
		// edge cases.
		//
		{
			name:               "No pieces => empty buffer, no aggregates",
			pieceSizes:         []int64{},
			expectedBufferSize: 0,
			expectedAggCount:   0,
		},
		{
			name: "Single piece ==64MB => triggers immediate aggregate (exact threshold)",
			// Exactly 64MB unpadded is already a power of two, so its padded size
			// is 128MB. That hits the newSize >= 128MB path inside AggregatePiece.
			pieceSizes:         []int64{64 * MB},
			expectedBufferSize: 0,
			expectedAggCount:   1,
		},
		{
			name: "Single piece just under 128MB => remains in buffer, no aggregates",
			// 63MB unpadded rounds up to 64MB padded, but it's still a single piece.
			pieceSizes:         []int64{63 * MB},
			expectedBufferSize: 64 * MB, // after rounding
			expectedAggCount:   0,
		},
		{
			name: "Single piece exactly 128MB => triggers immediate aggregate",
			// A piece right at the threshold gets flushed immediately.
			pieceSizes:         []int64{128 * MB},
			expectedBufferSize: 0,
			expectedAggCount:   1,
		},
		{
			name: "Single piece >128MB but <256MB => triggers immediate aggregate",
			// Because newPiece.PaddedSize() > 128MB => aggregator flushes right away.
			pieceSizes:         []int64{200 * MB},
			expectedBufferSize: 0,
			expectedAggCount:   1,
		},
		{
			name: "Single piece 192MB => triggers immediate aggregate",
			// By definition, a piece this large is also flushed right away as it pads to 256MB.
			pieceSizes:         []int64{192 * MB},
			expectedBufferSize: 0,
			expectedAggCount:   1,
		},
		{
			name: "Two small pieces that sum exactly 128MB => one aggregate, empty buffer",
			pieceSizes: []int64{
				63 * MB, // each might become ~64MB padded
				63 * MB, // combined crosses threshold
			},
			// The aggregator hits >=128MB on the second piece => flushes => buffer resets.
			expectedBufferSize: 0,
			expectedAggCount:   1,
		},
		{
			name: "Two small pieces slightly over 128MB => flush once, leftover in new buffer",
			pieceSizes: []int64{
				70 * MB, // each might become ~128MB padded
				70 * MB, // second piece triggers flush, but check leftover logic
			},
			// Because the first piece alone is >128MB padded, it flushes immediately,
			// leaving a fresh buffer that gets the second piece => second piece also
			// flushes. The net effect is typically two immediate aggregates if 70MB
			// rounds to 128MB. If you want them combined, pick sizes that together
			// cross 128 but not individually.
			expectedBufferSize: 0,
			expectedAggCount:   2,
		},
		{
			name: "Multiple pieces cause multiple flushes",
			pieceSizes: []int64{
				32 * MB, // ~64MB padded
				32 * MB, // crosses 128 => flush #1
				40 * MB, // alone remains in buffer since <128MB padded
				90 * MB, // once buffer + 90 crosses threshold => flush #2
				70 * MB, // new buffer, triggers flush #3
			},
			expectedBufferSize: 0,
			expectedAggCount:   3,
		},
		{
			name: "Single piece >256MB (if code permits) => immediate flush or error",
			// TODO(forrest): do we expect this to be an error case?
			// The aggregator code comment suggests >256MB is out-of-scope, but not
			// strictly enforced. Some implementations could treat this
			// as an error or just flush.
			pieceSizes:         []int64{300 * MB},
			expectedBufferSize: 0,
			expectedAggCount:   1,
		},
	}

	for _, tc := range tests {
		tc := tc // capture tc
		t.Run(tc.name, func(t *testing.T) {
			// NB(forrest): run these in parallel since creating MBs of data isn't exactly "fast".
			t.Parallel()
			var (
				buf        types.Buffer
				aggregates []types.Aggregate
				err        error
				pieces     []piece.PieceLink
			)

			// Build the input pieces
			for _, size := range tc.pieceSizes {
				pl := testutil.RandomPiece(t, size)
				pieces = append(pieces, pl)
			}

			// Call the function under test
			buf, aggregates, err = aggregator.AggregatePieces(buf, pieces)
			require.NoError(t, err, "AggregatePieces returned an unexpected error")

			// Check buffer size after all pieces are processed
			require.EqualValues(t, tc.expectedBufferSize, buf.TotalSize,
				"buffer size did not match expectation")

			// Check how many aggregates were formed
			if tc.expectedAggCount == 0 {
				require.Nil(t, aggregates,
					"expected no aggregates but got some")
			} else {
				require.Len(t, aggregates, tc.expectedAggCount,
					"number of aggregates does not match expectation")
			}
		})
	}
}
