package tasks

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/promise"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/store"
	"github.com/storacha/piri/pkg/store/blobstore"
)

var PieceParkPollInterval = time.Second

// ParkPieceTask gets a piece from some origin, and parks it in storage
// Pieces are always f00, piece ID is mapped to pieceCID in the DB
type ParkPieceTask struct {
	db *gorm.DB
	bs blobstore.Blobstore

	TF promise.Promise[scheduler.AddTaskFunc]

	// TODO delete this.
	longTerm bool // Indicates if the task is for long-term pieces
}

func NewStorePieceTask(db *gorm.DB, bs blobstore.Blobstore) *ParkPieceTask {
	return &ParkPieceTask{
		db:       db,
		bs:       bs,
		longTerm: true,
	}
}

func (p *ParkPieceTask) Start(ctx context.Context) {
	go p.pollPieceTasks(ctx)
}

func (p *ParkPieceTask) pollPieceTasks(ctx context.Context) {
	for {
		// Select parked pieces with no task_id and matching longTerm flag
		var pieces []models.ParkedPiece
		err := p.db.WithContext(ctx).
			Select("id").
			Where("long_term = ? AND complete = FALSE AND task_id IS NULL", p.longTerm).
			Find(&pieces).Error
		if err != nil {
			log.Errorf("failed to get parked pieces: %s", err)
			time.Sleep(PieceParkPollInterval)
			continue
		}

		if len(pieces) == 0 {
			time.Sleep(PieceParkPollInterval)
			continue
		}

		for _, piece := range pieces {
			// Create a task for each piece
			p.TF.Val(ctx)(func(id scheduler.TaskID, tx *gorm.DB) (shouldCommit bool, err error) {
				// Update
				res := tx.WithContext(ctx).Model(&models.ParkedPiece{}).
					Where("id = ? AND complete = FALSE AND task_id IS NULL AND long_term = ?", piece.ID, p.longTerm).
					Update("task_id", id)
				if res.Error != nil {
					return false, fmt.Errorf("updating parked piece: %w", res.Error)
				}
				// Commit only if we updated the piece
				return res.RowsAffected > 0, nil
			})
		}
	}
}

func (p *ParkPieceTask) Do(taskID scheduler.TaskID) (done bool, err error) {
	ctx := context.Background()

	// Select the piece data using the task ID and longTerm flag
	var piece []models.ParkedPiece
	err = p.db.WithContext(ctx).
		Where("task_id = ? AND long_term = ?", taskID, p.longTerm).
		First(&piece).Error
	if err != nil {
		return false, fmt.Errorf("fetching piece data: %w", err)
	}

	if len(piece) == 0 {
		return false, xerrors.Errorf("no piece data found for task_id: %d", taskID)
	}

	pieceData := piece[0]

	if pieceData.Complete {
		log.Warnw("park piece task already complete", "task_id", taskID, "piece_cid", pieceData.PieceCID)
		return true, nil
	}

	// Fetch reference data
	var refs []models.ParkedPieceRef
	err = p.db.WithContext(ctx).
		Where("piece_id = ? AND data_url IS NOT NULL", pieceData.ID).
		Find(&refs).Error
	if err != nil {
		return false, fmt.Errorf("fetching reference data: %w", err)
	}

	if len(refs) == 0 {
		return false, xerrors.Errorf("no refs found for piece_id: %d", pieceData.ID)
	}

	var merr error

	for i := range refs {
		if refs[i].DataURL != "" {
			sr, err := store.OpenStashFromURL(refs[i].DataURL)
			if err != nil {
				return false, fmt.Errorf("reading stash url: %w", err)
			}
			defer func() {
				_ = sr.Close()
			}()
			c, err := cid.Decode(pieceData.PieceCID)
			if err != nil {
				return false, fmt.Errorf("decoding cid: %w", err)
			}
			if err := p.bs.Put(ctx, c.Hash(), uint64(pieceData.PieceRawSize), sr); err != nil {
				return false, fmt.Errorf("putting piece data into store: %w", err)
			}

			// Update the piece as complete after a successful write.
			err = p.db.WithContext(ctx).
				Model(&models.ParkedPiece{}).
				Where("id = ?", pieceData.ID).
				Updates(map[string]interface{}{
					"complete": true,
					"task_id":  nil,
				}).Error
			if err != nil {
				return false, fmt.Errorf("marking piece as complete: %w", err)
			}

			return true, merr
		}
	}

	// If no suitable data URL is found
	return false, xerrors.Errorf("no suitable data URL found for piece_id %d: %w", pieceData.ID, merr)
}

func (p *ParkPieceTask) CanAccept(ids []scheduler.TaskID, engine *scheduler.TaskEngine) (*scheduler.TaskID, error) {
	id := ids[0]
	return &id, nil
}

func (p *ParkPieceTask) TypeDetails() scheduler.TaskTypeDetails {
	taskName := "StorePiece"

	return scheduler.TaskTypeDetails{
		Name:        taskName,
		MaxFailures: 10,
		RetryWait: func(retries int) time.Duration {
			baseWait, maxWait := 5*time.Second, time.Minute
			mul := 1.5

			// Use math.Pow for exponential backoff
			wait := time.Duration(float64(baseWait) * math.Pow(mul, float64(retries)))

			// Ensure the wait time doesn't exceed maxWait
			if wait > maxWait {
				return maxWait
			}
			return wait
		},
	}
}

func (p *ParkPieceTask) Adder(taskFunc scheduler.AddTaskFunc) {
	p.TF.Set(taskFunc)
}

var _ scheduler.TaskInterface = &ParkPieceTask{}
