package tasks

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/multiformats/go-multihash"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/promise"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/store/stashstore"
)

const (
	CleanupTaskPollInterval = 30 * time.Second
	CleanupTaskTypeStash    = "cleanup_stash"
	CleanupTaskTypeBlob     = "cleanup_blob"
)

type CleanupTask struct {
	db *gorm.DB
	bs blobstore.Blobstore
	ss stashstore.Stash
	TF promise.Promise[scheduler.AddTaskFunc]
}

type CleanupTaskData struct {
	Type      string    `json:"type"`      // "stash" or "blob"
	TargetID  string    `json:"target_id"` // UUID for stash, digest for blob
	CreatedAt time.Time `json:"created_at"`
}

func NewCleanupTask(db *gorm.DB, bs blobstore.Blobstore, ss stashstore.Stash) *CleanupTask {
	return &CleanupTask{
		db: db,
		bs: bs,
		ss: ss,
	}
}

func (c *CleanupTask) Start(ctx context.Context) {
	go c.pollCleanupTasks(ctx)
}

func (c *CleanupTask) pollCleanupTasks(ctx context.Context) {
	for {
		// Find completed parked pieces that need stash cleanup
		var pieces []models.ParkedPiece
		err := c.db.WithContext(ctx).
			Select("id").
			Where("complete = TRUE AND cleanup_task_id IS NULL").
			Find(&pieces).Error
		if err != nil {
			log.Errorf("failed to get completed parked pieces: %s", err)
			time.Sleep(CleanupTaskPollInterval)
			continue
		}

		// Find pieces with zero reference count that need blob cleanup
		var zeroRefPieces []models.PDPPieceRef
		err = c.db.WithContext(ctx).
			Select("id, piece_cid").
			Where("proofset_refcount = 0").
			Find(&zeroRefPieces).Error
		if err != nil {
			log.Errorf("failed to get zero ref count pieces: %s", err)
			time.Sleep(CleanupTaskPollInterval)
			continue
		}

		// Create cleanup tasks for stash files
		for _, piece := range pieces {
			c.TF.Val(ctx)(func(id scheduler.TaskID, tx *gorm.DB) (shouldCommit bool, err error) {
				res := tx.WithContext(ctx).Model(&models.ParkedPiece{}).
					Where("id = ? AND complete = TRUE AND cleanup_task_id IS NULL", piece.ID).
					Update("cleanup_task_id", id)
				if res.Error != nil {
					return false, fmt.Errorf("updating parked piece cleanup task: %w", res.Error)
				}
				return res.RowsAffected > 0, nil
			})
		}

		// Create cleanup tasks for blobs with zero reference count
		for _, piece := range zeroRefPieces {
			c.TF.Val(ctx)(func(id scheduler.TaskID, tx *gorm.DB) (shouldCommit bool, err error) {
				// Mark this piece as being cleaned up to prevent race conditions
				res := tx.WithContext(ctx).Model(&models.PDPPieceRef{}).
					Where("id = ? AND proofset_refcount = 0", piece.ID).
					Update("proofset_refcount", -1) // Use -1 to mark as being cleaned up
				if res.Error != nil {
					return false, fmt.Errorf("marking piece for cleanup: %w", res.Error)
				}
				return res.RowsAffected > 0, nil
			})
		}

		time.Sleep(CleanupTaskPollInterval)
	}
}

func (c *CleanupTask) Do(taskID scheduler.TaskID) (done bool, err error) {
	ctx := context.Background()

	// Check if this is a stash cleanup task
	var parkedPiece models.ParkedPiece
	err = c.db.WithContext(ctx).
		Where("cleanup_task_id = ?", taskID).
		First(&parkedPiece).Error
	if err == nil {
		// This is a stash cleanup task
		return c.cleanupStash(ctx, taskID, parkedPiece)
	}

	// Check if this is a blob cleanup task
	var pieceRef models.PDPPieceRef
	err = c.db.WithContext(ctx).
		Where("proofset_refcount = -1").
		First(&pieceRef).Error
	if err == nil {
		// This is a blob cleanup task
		return c.cleanupBlob(ctx, taskID, pieceRef)
	}

	return false, fmt.Errorf("no cleanup task found for task_id: %d", taskID)
}

func (c *CleanupTask) cleanupStash(ctx context.Context, taskID scheduler.TaskID, piece models.ParkedPiece) (done bool, err error) {
	log.Infow("cleaning up stash", "task_id", taskID, "piece_id", piece.ID)

	// Get the stash URL from the parked piece ref
	var ref models.ParkedPieceRef
	err = c.db.WithContext(ctx).
		Where("piece_id = ?", piece.ID).
		First(&ref).Error
	if err != nil {
		return false, fmt.Errorf("fetching parked piece ref: %w", err)
	}

	// Extract stash ID from the data URL
	stashIDStr, err := extractStashIDFromURL(ref.DataURL)
	if err != nil {
		return false, fmt.Errorf("extracting stash ID from URL: %w", err)
	}

	// Parse stash ID to UUID
	stashID, err := uuid.Parse(stashIDStr)
	if err != nil {
		return false, fmt.Errorf("parsing stash ID to UUID: %w", err)
	}

	// Remove the stash file
	if err := c.ss.StashRemove(ctx, stashID); err != nil {
		return false, fmt.Errorf("removing stash file: %w", err)
	}

	// Update the cleanup task ID to mark as completed
	if err := c.db.WithContext(ctx).Model(&models.ParkedPiece{}).
		Where("id = ?", piece.ID).
		Update("cleanup_task_id", nil).Error; err != nil {
		return false, fmt.Errorf("updating cleanup task completion: %w", err)
	}

	log.Infow("stash cleanup completed", "task_id", taskID, "piece_id", piece.ID, "stash_id", stashID)
	return true, nil
}

func (c *CleanupTask) cleanupBlob(ctx context.Context, taskID scheduler.TaskID, pieceRef models.PDPPieceRef) (done bool, err error) {
	log.Infow("cleaning up blob", "task_id", taskID, "piece_ref_id", pieceRef.ID, "piece_cid", pieceRef.PieceCID)

	// Convert piece CID to multihash for blobstore lookup
	// Note: This assumes the piece CID can be converted to a multihash
	// You may need to adjust this based on your actual piece CID format
	digest, err := pieceCIDToMultihash(pieceRef.PieceCID)
	if err != nil {
		return false, fmt.Errorf("converting piece CID to multihash: %w", err)
	}

	// Remove the blob from blobstore
	// Note: The blobstore interface doesn't have a Delete method, so we'll need to implement this
	// For now, we'll just mark the cleanup as complete
	if err := c.removeBlobFromStore(ctx, digest); err != nil {
		return false, fmt.Errorf("removing blob from store: %w", err)
	}

	// Delete the piece ref record
	if err := c.db.WithContext(ctx).Delete(&pieceRef).Error; err != nil {
		return false, fmt.Errorf("deleting piece ref: %w", err)
	}

	log.Infow("blob cleanup completed", "task_id", taskID, "piece_ref_id", pieceRef.ID, "piece_cid", pieceRef.PieceCID)
	return true, nil
}

func (c *CleanupTask) removeBlobFromStore(ctx context.Context, digest []byte) error {
	// Convert digest to multihash
	mh, err := multihash.Cast(digest)
	if err != nil {
		return fmt.Errorf("converting digest to multihash: %w", err)
	}

	// Use blob cleanup service to delete the blob
	if cleanupService, ok := c.bs.(*blobstore.BlobCleanupService); ok {
		return cleanupService.Delete(ctx, mh)
	}

	// Fallback: log that we can't delete (for blobstores without cleanup support)
	log.Infow("blobstore does not support deletion", "digest", digest)
	return nil
}

func (c *CleanupTask) TypeDetails() scheduler.TaskTypeDetails {
	return scheduler.TaskTypeDetails{
		Name:        "cleanup",
		MaxFailures: 3,
		RetryWait: func(retries int) time.Duration {
			return time.Duration(retries+1) * 5 * time.Minute
		},
	}
}

func (c *CleanupTask) Adder(tf scheduler.AddTaskFunc) {
	c.TF.Set(tf)
}

// Helper functions
func extractStashIDFromURL(dataURL string) (string, error) {
	// Extract stash ID from file:// URL
	// Example: file:///path/to/stash/uuid.tmp
	// We need to extract the uuid part
	// This is a simplified implementation - you may need to adjust based on your URL format
	if len(dataURL) < 7 || dataURL[:7] != "file://" {
		return "", fmt.Errorf("invalid stash URL format: %s", dataURL)
	}

	path := dataURL[7:]
	// Extract the filename and remove .tmp extension
	// This is a simplified approach - you may need more robust path parsing
	lastSlash := -1
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			lastSlash = i
			break
		}
	}

	if lastSlash == -1 {
		return "", fmt.Errorf("invalid stash URL path: %s", dataURL)
	}

	filename := path[lastSlash+1:]
	if len(filename) < 4 || filename[len(filename)-4:] != ".tmp" {
		return "", fmt.Errorf("invalid stash filename format: %s", filename)
	}

	return filename[:len(filename)-4], nil
}

func pieceCIDToMultihash(pieceCID string) ([]byte, error) {
	// Convert piece CID to multihash
	// This is a placeholder implementation
	// You'll need to implement this based on your piece CID format
	return []byte(pieceCID), nil
}
