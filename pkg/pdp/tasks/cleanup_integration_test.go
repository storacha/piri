package tasks

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/store/stashstore"
)

// TestCleanupIntegration tests the end-to-end cleanup flow
func TestCleanupIntegration(t *testing.T) {
	// Set up test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate the database
	err = db.AutoMigrate(&models.Task{}, &models.ParkedPiece{}, &models.ParkedPieceRef{}, &models.PDPPieceRef{})
	require.NoError(t, err)

	// Create test stash store
	stashStore, err := stashstore.NewStashStore("/tmp/test-stash")
	require.NoError(t, err)

	// Create test blobstore
	blobStore := blobstore.NewMapBlobstore()

	// Create cleanup task
	cleanupTask := NewCleanupTask(db, blobStore, stashStore)

	ctx := context.Background()

	// Test 1: Stash cleanup flow
	t.Run("StashCleanupFlow", func(t *testing.T) {
		// Create a test piece
		piece := models.ParkedPiece{
			PieceCID:        "baga6ea4seqa",
			PiecePaddedSize: 1024,
			PieceRawSize:    1000,
			Complete:        true,
		}
		err := db.Create(&piece).Error
		require.NoError(t, err)

		// Create a stash file
		stashID := uuid.New()
		stashURL, err := stashStore.StashURL(stashID)
		require.NoError(t, err)

		// Create parked piece ref
		ref := models.ParkedPieceRef{
			PieceID: piece.ID,
			DataURL: stashURL.String(),
		}
		err = db.Create(&ref).Error
		require.NoError(t, err)

		// Verify piece exists and needs cleanup
		var count int64
		err = db.Model(&models.ParkedPiece{}).
			Where("complete = TRUE AND cleanup_task_id IS NULL").
			Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)

		// Start cleanup task
		cleanupTask.Start(ctx)

		// Wait for cleanup to complete
		time.Sleep(100 * time.Millisecond)

		// Verify cleanup task was assigned
		err = db.Model(&models.ParkedPiece{}).
			Where("id = ?", piece.ID).
			Select("cleanup_task_id").
			Scan(&piece).Error
		require.NoError(t, err)
		assert.NotNil(t, piece.CleanupTaskID)
	})

	// Test 2: Blob cleanup flow
	t.Run("BlobCleanupFlow", func(t *testing.T) {
		// Create a piece ref with zero reference count
		pieceRef := models.PDPPieceRef{
			Service:          "test-service",
			PieceCID:         "baga6ea4seqa",
			ProofsetRefcount: 0,
		}
		err := db.Create(&pieceRef).Error
		require.NoError(t, err)

		// Verify piece ref exists with zero reference count
		var count int64
		err = db.Model(&models.PDPPieceRef{}).
			Where("proofset_refcount = 0").
			Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)

		// Start cleanup task
		cleanupTask.Start(ctx)

		// Wait for cleanup to complete
		time.Sleep(100 * time.Millisecond)

		// Verify piece ref was marked for cleanup (refcount = -1)
		err = db.Model(&models.PDPPieceRef{}).
			Where("id = ?", pieceRef.ID).
			Select("proofset_refcount").
			Scan(&pieceRef).Error
		require.NoError(t, err)
		assert.Equal(t, int64(-1), pieceRef.ProofsetRefcount)
	})

	// Test 3: Error handling
	t.Run("ErrorHandling", func(t *testing.T) {
		// Create a piece with invalid stash URL
		piece := models.ParkedPiece{
			PieceCID:        "baga6ea4seqa2",
			PiecePaddedSize: 1024,
			PieceRawSize:    1000,
			Complete:        true,
		}
		err := db.Create(&piece).Error
		require.NoError(t, err)

		// Create parked piece ref with invalid URL
		ref := models.ParkedPieceRef{
			PieceID: piece.ID,
			DataURL: "invalid://url",
		}
		err = db.Create(&ref).Error
		require.NoError(t, err)

		// Start cleanup task
		cleanupTask.Start(ctx)

		// Wait for cleanup to complete
		time.Sleep(100 * time.Millisecond)

		// Verify cleanup task was still assigned (error handling worked)
		err = db.Model(&models.ParkedPiece{}).
			Where("id = ?", piece.ID).
			Select("cleanup_task_id").
			Scan(&piece).Error
		require.NoError(t, err)
		assert.NotNil(t, piece.CleanupTaskID)
	})
}

// TestCleanupTaskTypeDetails tests the task type details
func TestCleanupTaskTypeDetails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	blobStore := blobstore.NewMapBlobstore()
	stashStore, err := stashstore.NewStashStore("/tmp/test-stash")
	require.NoError(t, err)

	cleanupTask := NewCleanupTask(db, blobStore, stashStore)

	details := cleanupTask.TypeDetails()
	assert.Equal(t, "cleanup", details.Name)
	assert.Equal(t, uint(3), details.MaxFailures)
	assert.NotNil(t, details.RetryWait)
}

// TestCleanupTaskAdder tests the task adder functionality
func TestCleanupTaskAdder(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	blobStore := blobstore.NewMapBlobstore()
	stashStore, err := stashstore.NewStashStore("/tmp/test-stash")
	require.NoError(t, err)

	cleanupTask := NewCleanupTask(db, blobStore, stashStore)

	// Test that adder can be set
	addTaskFunc := func(extraInfo func(scheduler.TaskID, *gorm.DB) (bool, error)) {}
	cleanupTask.Adder(addTaskFunc)

	// Verify the adder was set
	assert.True(t, cleanupTask.TF.IsSet())
}
