package comper

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"
	"time"

	logging "github.com/ipfs/go-log/v2"
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

var log = logging.Logger("comper")

var CommperModule = fx.Module("commper",
	fx.Provide(
		NewComper,
		NewCommpQueue,
		NewComperTaskHandler,
	),
)

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
		jobqueue.WithMaxTimeout(time.Minute),
		// TODO(forrest): might want to add extension timeout since this will take more than the default of 5 seconds
		// TODO(forrest): will likley need an on failure function
		// in the event computing a commp of the data fails, it is unclear what we shoud do
		// failure means it never makes it on chain, but the node will still be storing it
		// and the client will still be able to retirve it, honestly its a very concerning error
		// we could delete it from the datastore, but no way to inform a client their data is gone
		// or we could keep it and then the node is storing data it's not being paid for
		// though it can still egress it, tough decision..... maybe just retry for Uint64 Max? :')
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
	if err := c.queue.Register(CommpTaskName, params.Handler.Handle); err != nil {
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
