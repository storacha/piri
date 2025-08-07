package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"golang.org/x/xerrors"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service/models"
)

var _ scheduler.TaskInterface = &PDPNotifyTask{}

type PDPNotifyTask struct {
	db *gorm.DB
}

func NewPDPNotifyTask(db *gorm.DB) *PDPNotifyTask {
	return &PDPNotifyTask{db: db}
}

func (t *PDPNotifyTask) Do(taskID scheduler.TaskID) (done bool, err error) {
	ctx := context.Background()

	// Fetch the pdp_piece_uploads entry associated with the taskID
	var upload models.PDPPieceUpload
	err = t.db.WithContext(ctx).Where("notify_task_id = ?", taskID).First(&upload).Error
	if err != nil {
		return false, fmt.Errorf("failed to query pdp_piece_uploads for task %d: %w", taskID, err)
	}

	// Perform HTTP Post request to the notify URL
	upJson, err := json.Marshal(upload)
	if err != nil {
		return false, fmt.Errorf("failed to marshal upload to JSON: %w", err)
	}

	log.Infow("PDP notify", "upload", upload, "task_id", taskID)

	if upload.NotifyURL != "" {

		resp, err := http.Post(upload.NotifyURL, "application/json", bytes.NewReader(upJson))
		if err != nil {
			log.Errorw("HTTP POST request to notify_url failed", "notify_url", upload.NotifyURL, "upload_id", upload.ID, "error", err)
		} else {
			defer resp.Body.Close()
			// Not reading the body as per requirement
			log.Infow("HTTP GET request to notify_url succeeded", "notify_url", upload.NotifyURL, "upload_id", upload.ID)
		}
	}

	// Move the entry from pdp_piece_uploads to pdp_piecerefs
	// Insert into pdp_piecerefs
	ref := models.PDPPieceRef{
		Service:  upload.Service,
		PieceCID: *upload.PieceCID,
		PieceRef: *upload.PieceRef,
	}
	err = t.db.WithContext(ctx).Create(&ref).Error
	if err != nil {
		return false, fmt.Errorf("failed to insert into pdp_piecerefs: %w", err)
	}

	// Delete the entry from pdp_piece_uploads
	err = t.db.WithContext(ctx).Delete(&models.PDPPieceUpload{}, "id = ?", upload.ID).Error
	if err != nil {
		return false, fmt.Errorf("failed to delete upload ID %s from pdp_piece_uploads: %w", upload.ID, err)
	}

	log.Infof("Successfully processed PDP notify task %d for upload ID %s", taskID, upload.ID)

	return true, nil
}

func (t *PDPNotifyTask) CanAccept(ids []scheduler.TaskID, engine *scheduler.TaskEngine) (*scheduler.TaskID, error) {
	if len(ids) == 0 {
		return nil, xerrors.Errorf("no task IDs provided")
	}
	id := ids[0]
	return &id, nil
}

func (t *PDPNotifyTask) TypeDetails() scheduler.TaskTypeDetails {
	return scheduler.TaskTypeDetails{
		Name:        "PDPNotify",
		MaxFailures: 14,
		RetryWait: func(retries int) time.Duration {
			return time.Duration(float64(5*time.Second) * math.Pow(2, float64(retries)))
		},
		// Use the new PeriodicScheduler with helper function
		PeriodicScheduler: scheduler.NewPeriodicScheduler(time.Second, func(taskFunc scheduler.AddTaskFunc) error {
			return t.schedule(context.Background(), taskFunc)
		}),
	}
}

func (t *PDPNotifyTask) schedule(ctx context.Context, taskFunc scheduler.AddTaskFunc) error {
	var stop bool
	for !stop {
		taskFunc(func(id scheduler.TaskID, tx *gorm.DB) (shouldCommit bool, seriousError error) {
			stop = true // Assume we're done unless we find more tasks to schedule

			// Query for pending notifications where:
			// - piece_ref is not null
			// - The piece_ref points to a parked_piece_refs entry
			// - The parked_piece_refs entry points to a parked_pieces entry where complete = TRUE
			// - notify_task_id is NULL

			var uploads []models.PDPPieceUpload
			err := tx.WithContext(ctx).
				Model(&models.PDPPieceUpload{}).
				Joins("JOIN parked_piece_refs pr ON pr.ref_id = pdp_piece_uploads.piece_ref").
				Joins("JOIN parked_pieces pp ON pp.id = pr.piece_id").
				Where("pdp_piece_uploads.piece_ref IS NOT NULL").
				Where("pp.complete = ?", true).
				Where("pdp_piece_uploads.notify_task_id IS NULL").
				Limit(1).
				Select("pdp_piece_uploads.id").
				Find(&uploads).Error
			if err != nil {
				return false, fmt.Errorf("getting uploads to notify: %w", err)
			}

			if len(uploads) == 0 {
				// No uploads to process
				return false, nil
			}

			// Update the pdp_piece_uploads entry to set notify_task_id
			err = tx.WithContext(ctx).
				Model(&models.PDPPieceUpload{}).
				Where("id = ? AND notify_task_id IS NULL", uploads[0].ID).
				Update("notify_task_id", id).Error
			if err != nil {
				return false, fmt.Errorf("updating notify_task_id: %w", err)
			}

			stop = false     // Continue scheduling as there might be more tasks
			return true, nil // Commit the transaction
		})
	}
	return nil
}

func (t *PDPNotifyTask) Adder(taskFunc scheduler.AddTaskFunc) {
}
