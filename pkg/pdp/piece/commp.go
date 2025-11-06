package piece

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"

	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/multiformats/go-multihash"
	captypes "github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/piece/piece"
	"github.com/storacha/piri/pkg/pdp/aggregator"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/serializer"
	"github.com/storacha/piri/pkg/pdp/types"
	"go.uber.org/fx"
)

type Calculator interface {
	Enqueue(ctx context.Context, blob multihash.Multihash) error
}

type CommpQueueParams struct {
	fx.In
	DB *sql.DB `name:"aggregator_db"`
}

const (
	CommpQueueName = "commp"
	CommpTaskName  = "compute_commp"
)

func NewCommpQueue(lc fx.Lifecycle, params CommpQueueParams) (jobqueue.Service[multihash.Multihash], error) {
	commpQueue, err := jobqueue.New[multihash.Multihash](
		CommpTaskName,
		params.DB,
		&serializer.IPLDCBOR[multihash.Multihash]{
			Typ:  &schema.TypeBytes{},
			Opts: captypes.Converters,
		},
		jobqueue.WithLogger(log.With("queue", CommpQueueName)),
		// TODO(forrest): determine appropriate amount of retries
		// failures likely stem from failure to read from blobstore, of invalid cid inputs?
		jobqueue.WithMaxRetries(50),
		// TODO(forrest): number of worker will likely be a function of system memory + CPU
		jobqueue.WithMaxWorkers(uint(runtime.NumCPU())),
		// TODO(forrest): no idea how long commp will take in practice as it depends on
		// diskIO for reading, CPU for commp'in and RAM
	)
	if err != nil {
		return nil, fmt.Errorf("creating commp queue: %w", err)
	}

	queueCtx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return commpQueue.Start(queueCtx)
		},
		OnStop: func(ctx context.Context) error {
			cancel()
			return commpQueue.Stop(ctx)
		},
	})

	return commpQueue, nil
}

type TaskHandler interface {
	Handle(ctx context.Context, blob multihash.Multihash) error
}

type ComperParams struct {
	fx.In

	Queue   jobqueue.Service[multihash.Multihash]
	Handler TaskHandler
}

type Comper struct {
	queue jobqueue.Service[multihash.Multihash]
}

func NewComper(params ComperParams) (Calculator, error) {
	c := &Comper{
		queue: params.Queue,
	}
	if err := c.queue.Register(
		CommpTaskName,
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
	return c, nil
}

func (c *Comper) Enqueue(ctx context.Context, blob multihash.Multihash) error {
	return c.queue.Enqueue(ctx, CommpTaskName, blob)
}

func NewComperTaskHandler(api types.PieceAPI, a aggregator.Aggregator) TaskHandler {
	return &ComperTaskHandler{api: api, aggregator: a}
}

type ComperTaskHandler struct {
	api        types.PieceAPI
	aggregator aggregator.Aggregator
}

func (h *ComperTaskHandler) Handle(ctx context.Context, blob multihash.Multihash) error {
	pieceCid, err := h.api.CalculateCommP(ctx, blob)
	if err != nil {
		return err
	}
	p, err := piece.FromLink(cidlink.Link{Cid: pieceCid})
	if err != nil {
		return err
	}
	return h.aggregator.AggregatePiece(ctx, p)
}
