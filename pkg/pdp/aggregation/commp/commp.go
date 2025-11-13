package commp

import (
	"context"
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	"github.com/multiformats/go-multihash"
	"go.uber.org/fx"

	"github.com/storacha/piri/lib/jobqueue"
)

var log = logging.Logger("aggregation/commp")

type Calculator interface {
	Enqueue(ctx context.Context, blob multihash.Multihash) error
}

type ComperParams struct {
	fx.In

	Queue   jobqueue.Service[multihash.Multihash]
	Handler jobqueue.TaskHandler[multihash.Multihash]
}

type Comper struct {
	queue jobqueue.Service[multihash.Multihash]
}

func NewQueuingCommpCalculator(lc fx.Lifecycle, params ComperParams) (Calculator, error) {
	c := &Comper{
		queue: params.Queue,
	}
	if err := c.queue.Register(
		TaskName,
		params.Handler.Handle,
		jobqueue.WithOnFailure(func(ctx context.Context, msg multihash.Multihash, err error) error {
			// NB(forrest): failed tasks will go to the dead-letter queue, meaning the failure is detectable,
			// and could be retried later.
			// TODO(forrest): in the very rare case a failure happens, node operators may want to take action by:
			// 1. Telling a developer!
			// 2. Manually deleting the data from their store, since failure here prevents a root from being added,
			// thus no payment for its storage.
			// 3. Communicate to the client that this data is no longer being stored for the client.
			// 4. Manually retry the failed task from the job queue.
			log.Errorw("failed to handle commp task", "multihash", msg.String(), "err", err)
			return nil
		})); err != nil {
		return nil, fmt.Errorf("registering comper task handler: %w", err)
	}

	queueCtx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return c.queue.Start(queueCtx)
		},
		OnStop: func(ctx context.Context) error {
			cancel()
			return c.queue.Stop(ctx)
		},
	})

	return c, nil
}

func (c *Comper) Enqueue(ctx context.Context, blob multihash.Multihash) error {
	log.Infow("enqueuing commp", "blob", blob.String())
	return c.queue.Enqueue(ctx, TaskName, blob)
}
