package retrievaljournal

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
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

		journal, err := NewFSJournal(tempDir, 0) // Default batch size
		require.NoError(t, err)

		// Create a test receipt
		rcpt := createTestReceipt(t)

		// Append a receipt
		_, _, err = journal.Append(t.Context(), rcpt)
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
		journal, err := NewFSJournal(tempDir, 1024) // 1KB batches
		require.NoError(t, err)

		// Create a few test receipts
		var rcpts []receipt.Receipt[content.RetrieveOk, fdm.FailureModel]
		for range 10 {
			rcpts = append(rcpts, createTestReceipt(t))
		}

		// Append receipts and fill batches
		numBatches := 0
		currentBatchSize := 18 // 18 bytes is the CAR header size
		for _, rcpt := range rcpts {
			batchRotated, _, err := journal.Append(t.Context(), rcpt)
			require.NoError(t, err)

			archive := rcpt.Archive()
			archBytes, err := io.ReadAll(archive)
			require.NoError(t, err)
			currentBatchSize += len(archBytes) + 39 // 39 bytes is the overhead per block in the CAR file

			if int64(currentBatchSize) >= journal.maxBatchSize {
				// Check that batches are flushed when they reach the max size
				require.True(t, batchRotated)
				currentBatchSize = 18
				numBatches++
				require.Equal(t, int64(currentBatchSize), journal.currSize)

				files, err := filepath.Glob(filepath.Join(tempDir, batchFilePrefix+"*"+batchFileSuffix))
				require.NoError(t, err)
				require.Len(t, files, numBatches, "expected %d completed batch files", numBatches)
			} else {
				require.False(t, batchRotated)
			}
		}
	})

	t.Run("concurrent append", func(t *testing.T) {
		tempDir := t.TempDir()

		journal, err := NewFSJournal(tempDir, 1024)
		require.NoError(t, err)

		var wg sync.WaitGroup
		numReceipts := 10

		// Create multiple goroutines to append receipts concurrently
		numBatches := atomic.Int32{}
		for range numReceipts {
			wg.Add(1)
			go func() {
				defer wg.Done()
				rcpt := createTestReceipt(t)
				batchRotated, _, err := journal.Append(t.Context(), rcpt)
				require.NoError(t, err)

				if batchRotated {
					numBatches.Add(1)
				}
			}()
		}

		wg.Wait()

		files, err := filepath.Glob(filepath.Join(tempDir, batchFilePrefix+"*"+batchFileSuffix))
		require.NoError(t, err)
		n := int(numBatches.Load())
		require.Len(t, files, n, "expected %d completed batch files", n)
	})

	t.Run("fails with nil receipt", func(t *testing.T) {
		tempDir := t.TempDir()

		journal, err := NewFSJournal(tempDir, 0) // Default batch size
		require.NoError(t, err)

		_, _, err = journal.Append(t.Context(), nil)
		require.Error(t, err)
	})
}

func TestRotate(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		tempDir := t.TempDir()

		journal, err := NewFSJournal(tempDir, 0)
		require.NoError(t, err)

		// Create a test receipt
		rcpt := createTestReceipt(t)

		// Append the receipt. A single receipt is not enough to trigger a rotation.
		batchRotated, _, err := journal.Append(t.Context(), rcpt)
		require.NoError(t, err)
		require.False(t, batchRotated)

		// Rotate the batch
		rotatedBatchCID, err := journal.rotate()
		require.NoError(t, err)
		require.NotEmpty(t, rotatedBatchCID)

		// Check that the batch file was created
		files, err := filepath.Glob(filepath.Join(tempDir, fmt.Sprintf("%s%s%s", batchFilePrefix, rotatedBatchCID.String(), batchFileSuffix)))
		require.NoError(t, err)
		require.Len(t, files, 1, "expected one batch file")
	})
}

func TestGetBatch(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		tempDir := t.TempDir()

		journal, err := NewFSJournal(tempDir, 100)
		require.NoError(t, err)

		// Create a test receipt
		rcpt := createTestReceipt(t)

		// Append the receipt. Max batch size is small, so a batch should be rotated.
		batchRotated, rotatedBatchCID, err := journal.Append(t.Context(), rcpt)
		require.NoError(t, err)
		require.True(t, batchRotated)

		// Read the batch file directly from the file system
		f, err := os.Open(filepath.Join(tempDir, fmt.Sprintf("egress.%s.car", rotatedBatchCID.String())))
		require.NoError(t, err)
		readBytes, err := io.ReadAll(f)
		require.NoError(t, err)

		// Get the batch
		batch, err := journal.GetBatch(t.Context(), rotatedBatchCID)
		require.NoError(t, err)

		// Read the batch and compare with file contents
		batchBytes, err := io.ReadAll(batch)
		require.NoError(t, err)

		require.True(t, slices.Equal(readBytes, batchBytes))
	})
}

func TestResumeAfterRestart(t *testing.T) {
	t.Run("hash is computed correctly after restart", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create initial journal and append some receipts
		journal1, err := NewFSJournal(tempDir, DefaultBatchSize)
		require.NoError(t, err)

		rcpt1 := createTestReceipt(t)
		rcpt2 := createTestReceipt(t)

		rotated, batch, err := journal1.Append(t.Context(), rcpt1)
		require.NoError(t, err)
		require.False(t, rotated)
		require.Equal(t, cid.Undef, batch)

		rotated, batch, err = journal1.Append(t.Context(), rcpt2)
		require.NoError(t, err)
		require.False(t, rotated)
		require.Equal(t, cid.Undef, batch)

		// Close the journal (simulating restart)
		err = journal1.Close()
		require.NoError(t, err)

		// Reopen the journal (this should resume the existing batch)
		journal2, err := NewFSJournal(tempDir, DefaultBatchSize)
		require.NoError(t, err)

		// Append another receipt then force rotation
		rcpt3 := createTestReceipt(t)
		rotated, batch, err = journal2.Append(t.Context(), rcpt3)
		require.NoError(t, err)
		require.False(t, rotated)
		require.Equal(t, cid.Undef, batch)

		// Force rotation to get the batch CID
		rotatedBatchCID, err := journal2.rotate()
		require.NoError(t, err)

		// Use GetBatch to retrieve the batch
		batchReader, err := journal2.GetBatch(t.Context(), rotatedBatchCID)
		require.NoError(t, err)
		defer batchReader.Close()

		batchData, err := io.ReadAll(batchReader)
		require.NoError(t, err)

		// Compute the expected CID from the actual file contents
		expectedHash := sha256.Sum256(batchData)
		expectedMhBytes, _ := multihash.Encode(expectedHash[:], multihash.SHA2_256)
		expectedMh := multihash.Multihash(expectedMhBytes)
		expectedCID := cid.NewCidV1(uint64(multicodec.Car), expectedMh)

		// The rotated batch CID should match the hash of the entire file
		require.Equal(t, expectedCID.String(), rotatedBatchCID.String(),
			"batch CID should be the hash of the entire file, including data written before restart")

		err = journal2.Close()
		require.NoError(t, err)
	})
}
