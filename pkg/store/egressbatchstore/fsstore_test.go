package egressbatchstore

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/storacha/go-libstoracha/capabilities/space/content"
	captypes "github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/core/receipt/ran"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/result/failure"
	fdm "github.com/storacha/go-ucanto/core/result/failure/datamodel"
	"github.com/stretchr/testify/require"
)

func createTestReceipt(t *testing.T) receipt.Receipt[content.RetrieveOk, fdm.FailureModel] {
	client := testutil.Alice
	node := testutil.Service
	space := testutil.RandomDID(t)
	inv, err := content.Retrieve.Invoke(
		client,
		node,
		space.String(),
		content.RetrieveCaveats{
			Blob: content.BlobDigest{
				Digest: testutil.RandomMultihash(t),
			},
			Range: content.Range{
				Start: 1024,
				End:   2048,
			},
		},
	)
	require.NoError(t, err)

	ran := ran.FromInvocation(inv)
	ok := result.Ok[content.RetrieveOk, failure.IPLDBuilderFailure](content.RetrieveOk{})
	rcpt, err := receipt.Issue(
		node,
		ok,
		ran,
	)
	require.NoError(t, err)

	retrieveRcpt, err := receipt.Rebind[content.RetrieveOk, fdm.FailureModel](rcpt, content.RetrieveOkType(), fdm.FailureType(), captypes.Converters...)
	require.NoError(t, err)

	return retrieveRcpt
}

func TestAppend(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		tempDir := t.TempDir()

		store, err := NewFSBatchStore(tempDir, 0) // Default batch size
		require.NoError(t, err)

		// Create a test receipt
		rcpt := createTestReceipt(t)

		// Append a receipt
		err = store.Append(context.Background(), rcpt)
		require.NoError(t, err)

		// Verify the file exists and has content
		files, err := filepath.Glob(filepath.Join(tempDir, "egress.car"))
		require.NoError(t, err)
		require.Len(t, files, 1, "expected one batch file")

		fileInfo, err := os.Stat(files[0])
		require.NoError(t, err)
		require.True(t, fileInfo.Size() > 0, "batch file should not be empty")

		// Test reading the file back
		data, err := os.ReadFile(files[0])
		require.NoError(t, err)
		require.True(t, len(data) > 0, "should be able to read file content")
	})

	t.Run("batch is flushed when max batch size is reached", func(t *testing.T) {
		tempDir := t.TempDir()

		// Small batch size to force rotation
		store, err := NewFSBatchStore(tempDir, 1024) // 1KB batches
		require.NoError(t, err)

		// Create a few test receipts
		var rcpts []receipt.Receipt[content.RetrieveOk, fdm.FailureModel]
		for range 10 {
			rcpts = append(rcpts, createTestReceipt(t))
		}

		// Append receipts and fill batches
		numBatches := 0
		currentBatchSize := 0
		for _, rcpt := range rcpts {
			err = store.Append(context.Background(), rcpt)
			require.NoError(t, err)

			archive := rcpt.Archive()
			archBytes, err := io.ReadAll(archive)
			require.NoError(t, err)
			currentBatchSize += len(archBytes)

			if int64(currentBatchSize) >= store.maxBatchSize {
				// Check that batches are flushed when they reach the max size
				currentBatchSize = 0
				numBatches++
				reportedBatchSize, err := store.currentBatchSize()
				require.NoError(t, err)
				require.Equal(t, int64(0), reportedBatchSize)

				files, err := filepath.Glob(filepath.Join(tempDir, "egress.*.car"))
				require.NoError(t, err)
				require.Len(t, files, numBatches, "expected %d completed batch files", numBatches)
			}
		}
	})

	t.Run("concurrent appends", func(t *testing.T) {
		tempDir := t.TempDir()
		store, err := NewFSBatchStore(tempDir, 1024) // 1KB
		require.NoError(t, err)

		var wg sync.WaitGroup
		numReceipts := 10

		// Create multiple goroutines to append receipts concurrently
		for range numReceipts {
			wg.Add(1)
			go func() {
				defer wg.Done()
				rcpt := createTestReceipt(t)
				err := store.Append(context.Background(), rcpt)
				require.NoError(t, err)
			}()
		}

		wg.Wait()

		// Verify we have some data written
		files, err := filepath.Glob(filepath.Join(tempDir, "egress.*.car"))
		require.NoError(t, err)
		require.True(t, len(files) > 0, "expected at least one batch file")
	})

	t.Run("fails with nil receipt", func(t *testing.T) {
		tempDir := t.TempDir()

		store, err := NewFSBatchStore(tempDir, 0) // Default batch size
		require.NoError(t, err)

		err = store.Append(context.Background(), nil)
		require.Error(t, err)
	})
}

func TestFlush(t *testing.T) {
	tempDir := t.TempDir()

	store, err := NewFSBatchStore(tempDir, 0) // Default batch size
	require.NoError(t, err)

	// Create a test receipt
	rcpt := createTestReceipt(t)

	// Append the receipt. A single receipt is not enough to trigger a flush.
	err = store.Append(context.Background(), rcpt)
	require.NoError(t, err)

	// Flush the batch
	err = store.Flush(context.Background())
	require.NoError(t, err)

	// Check that the batch file was created
	files, err := filepath.Glob(filepath.Join(tempDir, "egress.*.car"))
	require.NoError(t, err)
	require.Len(t, files, 1, "expected one batch file")

	t.Run("flushing empty store is a no-op", func(t *testing.T) {
		tempDir := t.TempDir()

		store, err := NewFSBatchStore(tempDir, 0) // Default batch size
		require.NoError(t, err)

		err = store.Flush(context.Background())
		require.NoError(t, err, "flushing empty store should not error")
	})
}
