package retrievaljournal

import (
	"context"
	"time"

	"github.com/ipfs/go-cid"
)

type PeriodicRotator struct {
	journal  ForceRotator
	period   time.Duration
	stopping chan struct{}
	stopped  chan struct{}
	// RotateFunc is is called when a rotation occurs with the CID of the rotated
	// batch. This can be used to trigger post-rotation actions, such as enqueuing
	// a task to process the batch.
	RotateFunc func(cid.Cid)
}

func NewPeriodicRotator(journal ForceRotator, period time.Duration) *PeriodicRotator {
	return &PeriodicRotator{
		journal:  journal,
		period:   period,
		stopping: make(chan struct{}),
		stopped:  make(chan struct{}),
	}
}

func (r *PeriodicRotator) Start() {
	go r.run()
}

func (r *PeriodicRotator) run() {
	ticker := time.NewTicker(r.period)
	defer ticker.Stop()
	defer close(r.stopped)

	for {
		select {
		case <-r.stopping:
			return
		case <-ticker.C:
			rotated, batchID, err := r.journal.ForceRotate(context.Background())
			if err != nil {
				// Log the error but continue with the next rotation attempt
				log.Errorw("forcing journal rotation", "error", err)
			} else if rotated {
				log.Infow("journal rotated", "batch", batchID)
				if r.RotateFunc != nil {
					r.RotateFunc(batchID)
				}
			}

		}
	}
}

func (r *PeriodicRotator) Stop(ctx context.Context) error {
	close(r.stopping)
	select {
	case <-r.stopped:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
