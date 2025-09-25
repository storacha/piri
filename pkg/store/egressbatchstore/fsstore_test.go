package egressbatchstore

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
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
		_, _, err = store.Append(t.Context(), rcpt)
		require.NoError(t, err)

		// Verify the file for the current batch exists and has content
		files, err := filepath.Glob(filepath.Join(tempDir, currentBatchName))
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

	t.Run("batch is rotated when max batch size is reached", func(t *testing.T) {
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
			batchRotated, _, err := store.Append(t.Context(), rcpt)
			require.NoError(t, err)

			archive := rcpt.Archive()
			archBytes, err := io.ReadAll(archive)
			require.NoError(t, err)
			currentBatchSize += len(archBytes)

			if int64(currentBatchSize) >= store.maxBatchSize {
				// Check that batches are flushed when they reach the max size
				require.True(t, batchRotated)
				currentBatchSize = 0
				numBatches++
				reportedBatchSize, err := store.currentBatchSize()
				require.NoError(t, err)
				require.Equal(t, int64(0), reportedBatchSize)

				files, err := filepath.Glob(filepath.Join(tempDir, batchFilePrefix+"*"+batchFileSuffix))
				require.NoError(t, err)
				require.Len(t, files, numBatches, "expected %d completed batch files", numBatches)
			} else {
				require.False(t, batchRotated)
			}
		}
	})

	t.Run("fails with nil receipt", func(t *testing.T) {
		tempDir := t.TempDir()

		store, err := NewFSBatchStore(tempDir, 0) // Default batch size
		require.NoError(t, err)

		_, _, err = store.Append(t.Context(), nil)
		require.Error(t, err)
	})
}

func TestRotate(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		tempDir := t.TempDir()

		store, err := NewFSBatchStore(tempDir, 0)
		require.NoError(t, err)

		// Create a test receipt
		rcpt := createTestReceipt(t)

		// Append the receipt. A single receipt is not enough to trigger a rotation.
		batchRotated, _, err := store.Append(t.Context(), rcpt)
		require.NoError(t, err)
		require.False(t, batchRotated)

		// Rotate the batch
		rotatedBatchCID, err := store.rotate()
		require.NoError(t, err)
		require.NotEmpty(t, rotatedBatchCID)

		// Check that the batch file was created
		files, err := filepath.Glob(filepath.Join(tempDir, fmt.Sprintf("%s%s%s", batchFilePrefix, rotatedBatchCID.String(), batchFileSuffix)))
		require.NoError(t, err)
		require.Len(t, files, 1, "expected one batch file")
	})

	t.Run("rotating empty store returns an error", func(t *testing.T) {
		tempDir := t.TempDir()

		store, err := NewFSBatchStore(tempDir, 0)
		require.NoError(t, err)

		_, err = store.rotate()
		require.Error(t, err)
	})
}

func TestGetBatch(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		tempDir := t.TempDir()

		store, err := NewFSBatchStore(tempDir, 100)
		require.NoError(t, err)

		// Create a test receipt
		rcpt := createTestReceipt(t)

		// Append the receipt. Max batch size is small, so a batch should be rotated.
		batchRotated, rotatedBatchCID, err := store.Append(t.Context(), rcpt)
		require.NoError(t, err)
		require.True(t, batchRotated)

		// Read the batch file directly from the file system
		f, err := os.Open(filepath.Join(tempDir, fmt.Sprintf("egress.%s.car", rotatedBatchCID.String())))
		require.NoError(t, err)
		readBytes, err := io.ReadAll(f)
		require.NoError(t, err)

		// Get the batch
		batch, err := store.GetBatch(t.Context(), rotatedBatchCID)
		require.NoError(t, err)

		// Read the batch and compare with file contents
		batchBytes, err := io.ReadAll(batch)
		require.NoError(t, err)

		require.True(t, slices.Equal(readBytes, batchBytes))
	})
}
