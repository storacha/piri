package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	commcid "github.com/filecoin-project/go-fil-commcid"
	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/singleflight"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// mockPieceReader is a mock implementation of types.PieceReaderAPI for testing
type mockPieceReader struct {
	data      map[string][]byte
	callCount atomic.Int32
	await     chan struct{}
}

func (m *mockPieceReader) Read(ctx context.Context, blob multihash.Multihash, options ...types.ReadPieceOption) (*types.PieceReader, error) {
	m.callCount.Add(1)

	if m.await != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-m.await:
		}
	}

	data, ok := m.data[blob.HexString()]
	if !ok {
		return nil, io.ErrUnexpectedEOF
	}

	return &types.PieceReader{
		Data: io.NopCloser(bytes.NewReader(data)),
		Size: int64(len(data)),
	}, nil
}

func (m *mockPieceReader) Has(ctx context.Context, blob multihash.Multihash) (bool, error) {
	_, ok := m.data[blob.HexString()]
	return ok, nil
}

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:memdb-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	sqlDb, err := db.DB()
	require.NoError(t, err)
	sqlDb.SetMaxOpenConns(1)
	err = db.AutoMigrate(&models.PDPPieceMHToCommp{})
	require.NoError(t, err)

	return db
}

func TestCalculateCommP_Singleflight(t *testing.T) {
	db := setupTestDB(t)
	synctest.Test(t, func(t *testing.T) {

		ctx := context.Background()

		// Create test data
		size := uint64(10 * 1024)
		data := testutil.RandomBytes(t, int(size))

		// Calculate the expected result
		c := &commp.Calc{}
		_, err := io.Copy(c, bytes.NewReader(data))
		require.NoError(t, err)

		commpDigest, expectedPaddedSize, err := c.Digest()
		require.NoError(t, err)
		expectedPieceCID, err := commcid.DataCommitmentToPieceCidv2(commpDigest, size)
		require.NoError(t, err)
		// Create multihash for the data
		blob := randomMultihash(t, data)

		// Create mock reader that tracks call count
		reader := &mockPieceReader{
			data: map[string][]byte{
				blob.HexString(): data,
			},
			await: make(chan struct{}),
		}

		// Create PDPService with just the fields needed for CalculateCommP
		service := &PDPService{
			db:          db,
			pieceReader: reader,
			commPGroup:  singleflight.Group{},
		}

		// Launch multiple concurrent calls to CalculateCommP with the same blob
		const numCalls = 10
		var wg sync.WaitGroup
		results := make([]types.CalculateCommPResponse, numCalls)
		errors := make([]error, numCalls)

		for i := range numCalls {
			wg.Go(func() {
				results[i], errors[i] = service.CalculateCommP(ctx, blob)
			})
		}

		synctest.Wait()
		close(reader.await) // let all reads proceed
		wg.Wait()

		// Verify all calls succeeded
		for i := range numCalls {
			require.NoError(t, errors[i], "call %d failed", i)
			require.NotEqual(t, cid.Undef, results[i].PieceCID, "call %d returned undefined CID", i)
		}

		// Verify all results are identical and match expected
		for i := range numCalls {
			require.Equal(t, expectedPieceCID.String(), results[i].PieceCID.String(), "results differ at index %d", i)
			require.Equal(t, int64(size), results[i].RawSize, "raw sizes differ at index %d", i)
			require.Equal(t, int64(expectedPaddedSize), results[i].PaddedSize, "padded sizes differ at index %d", i)
		}

		// CRITICAL: Verify that doCommp was only called ONCE despite 10 concurrent requests
		// This proves singleflight is working
		actualCalls := reader.callCount.Load()
		require.Equal(t, int32(1), actualCalls,
			"expected pieceReader.Read to be called exactly once due to singleflight, but was called %d times",
			actualCalls)
	})
}

func TestCalculateCommP_DifferentBlobs(t *testing.T) {
	db := setupTestDB(t)

	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		// Create two different blobs
		size := uint64(10 * 1024)
		data1 := testutil.RandomBytes(t, int(size))
		data2 := testutil.RandomBytes(t, int(size))

		blob1 := randomMultihash(t, data1)
		blob2 := randomMultihash(t, data2)

		reader := &mockPieceReader{
			data: map[string][]byte{
				blob1.HexString(): data1,
				blob2.HexString(): data2,
			},
			await: make(chan struct{}),
		}

		service := &PDPService{
			db:          db,
			pieceReader: reader,
			commPGroup:  singleflight.Group{},
		}

		// Launch concurrent calls for DIFFERENT blobs
		var wg sync.WaitGroup
		var result1, result2 types.CalculateCommPResponse
		var err1, err2 error

		wg.Add(2)
		go func() {
			defer wg.Done()
			result1, err1 = service.CalculateCommP(ctx, blob1)
		}()
		go func() {
			defer wg.Done()
			result2, err2 = service.CalculateCommP(ctx, blob2)
		}()

		synctest.Wait()
		close(reader.await) // let all reads proceed
		wg.Wait()

		require.NoError(t, err1)
		require.NoError(t, err2)

		// Results should be different since blobs are different
		require.NotEqual(t, result1.PieceCID.String(), result2.PieceCID.String())

		// Both blobs should have been read (singleflight shouldn't dedupe different keys)
		require.Equal(t, int32(2), reader.callCount.Load())
	})
}

func TestCalculateCommP_DatabaseCache(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	size := uint64(10 * 1024)
	data := testutil.RandomBytes(t, int(size))
	blob := randomMultihash(t, data)

	reader := &mockPieceReader{
		data: map[string][]byte{
			blob.HexString(): data,
		},
	}

	service := &PDPService{
		db:          db,
		pieceReader: reader,
		commPGroup:  singleflight.Group{},
	}

	// First call - should calculate and cache
	result1, err := service.CalculateCommP(ctx, blob)
	require.NoError(t, err)
	require.Equal(t, int32(1), reader.callCount.Load())

	// Second call - should hit database cache (not singleflight, since first call completed)
	result2, err := service.CalculateCommP(ctx, blob)
	require.NoError(t, err)

	// Results should be identical
	require.Equal(t, result1.PieceCID.String(), result2.PieceCID.String())
	require.Equal(t, result1.RawSize, result2.RawSize)
	require.Equal(t, result1.PaddedSize, result2.PaddedSize)

	// pieceReader should still only have been called once (second call used DB cache)
	require.Equal(t, int32(1), reader.callCount.Load())
}

func TestCalculateCommP_ConcurrentWithCache(t *testing.T) {
	db := setupTestDB(t)
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()

		size := uint64(10 * 1024)
		data := testutil.RandomBytes(t, int(size))
		blob := randomMultihash(t, data)

		reader := &mockPieceReader{
			data: map[string][]byte{
				blob.HexString(): data,
			},
		}

		service := &PDPService{
			db:          db,
			pieceReader: reader,
			commPGroup:  singleflight.Group{},
		}

		// First, populate the cache
		result1, err := service.CalculateCommP(ctx, blob)
		require.NoError(t, err)
		require.Equal(t, int32(1), reader.callCount.Load())

		reader.await = make(chan struct{}) // block further reads

		// Now make concurrent calls - they should all hit the DB cache
		const numCalls = 10
		var wg sync.WaitGroup
		results := make([]types.CalculateCommPResponse, numCalls)
		errors := make([]error, numCalls)

		for i := 0; i < numCalls; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				results[idx], errors[idx] = service.CalculateCommP(ctx, blob)
			}(i)
		}

		synctest.Wait()
		close(reader.await) // let all reads proceed
		wg.Wait()

		// All calls should succeed
		for i := 0; i < numCalls; i++ {
			require.NoError(t, errors[i])
			require.Equal(t, result1.PieceCID.String(), results[i].PieceCID.String())
		}

		// pieceReader should still only have been called once (all subsequent calls used DB cache)
		require.Equal(t, int32(1), reader.callCount.Load())
	})
}
