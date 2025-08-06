package aggregator

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-libstoracha/piece/piece"
	"go.uber.org/fx"

	"github.com/storacha/piri/internal/ipldstore"
	"github.com/storacha/piri/pkg/pdp/aggregator"
	"github.com/storacha/piri/pkg/pdp/aggregator/aggregate"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/serializer"
)

var Module = fx.Module("aggregator",
	fx.Provide(
		aggregator.New,
		aggregator.NewPieceAccepter,
		aggregator.NewAggregateSubmitteer,
		aggregator.NewPieceAggregator,
		ProvideStore,
		ProvideInProgressWorkspace,
		ProvidePieceQueue,
		ProvideLinkQueue, // TODO might need to decorate
	),

	fx.Invoke(
		RegisterPieceQueueJobs,
		RegisterLinkQueueJobs,
	),
)

const workspaceKey = "workspace/"
const aggregatePrefix = "aggregates/"

// queue args
const (
	LinkQueueName  = "link"
	PieceQueueName = "piece"

	PieceAggregateTask = "piece_aggregate"
	PieceSubmitTask    = "piece_submit"
	PieceAcceptTask    = "piece_accept"
)

type StoreParams struct {
	Datastore datastore.Datastore `name:"aggregator_datastore"`
}

func ProvideStore(params StoreParams) ipldstore.KVStore[datamodel.Link, aggregate.Aggregate] {
	return ipldstore.IPLDStore[datamodel.Link, aggregate.Aggregate](
		store.SimpleStoreFromDatastore(namespace.Wrap(params.Datastore, datastore.NewKey(aggregatePrefix))),
		aggregate.AggregateType(), types.Converters...,
	)
}

func ProvideInProgressWorkspace(params StoreParams) aggregator.InProgressWorkspace {
	return aggregator.NewInProgressWorkspace(store.SimpleStoreFromDatastore(namespace.Wrap(params.Datastore, datastore.NewKey(workspaceKey))))
}

type LinkQueueParams struct {
	DB *sql.DB `name:"aggregator_db"`
}

func ProvideLinkQueue(lc fx.Lifecycle, params LinkQueueParams) (*jobqueue.JobQueue[datamodel.Link], error) {
	linkQueue, err := jobqueue.New[datamodel.Link, aggregate.Aggregate](
		LinkQueueName,
		params.DB,
		&serializer.IPLDCBOR[datamodel.Link]{
			Typ:  &schema.TypeLink{},
			Opts: types.Converters,
		},
		jobqueue.WithLogger(logging.Logger("jobqueue").With("queue", LinkQueueName)),
		jobqueue.WithMaxRetries(50),
		jobqueue.WithMaxWorkers(uint(runtime.NumCPU())),
	)
	if err != nil {
		return nil, fmt.Errorf("creating link job-queue: %w", err)
	}
	queueCtx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			linkQueue.Start(queueCtx)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			cancel()
			return nil
		},
	})
	return linkQueue, nil
}

func ProvidePieceQueue(lc fx.Lifecycle, params LinkQueueParams) (*jobqueue.JobQueue[piece.PieceLink], error) {
	pieceQueue, err := jobqueue.New(
		PieceQueueName,
		params.DB,
		&serializer.IPLDCBOR[piece.PieceLink]{
			Typ:  aggregate.PieceLinkType(),
			Opts: types.Converters,
		},
		jobqueue.WithLogger(logging.Logger("jobqueue").With("queue", PieceQueueName)),
		jobqueue.WithMaxRetries(50),
		jobqueue.WithMaxWorkers(uint(runtime.NumCPU())),
	)
	if err != nil {
		return nil, fmt.Errorf("creating piece_link job-queue: %w", err)
	}
	queueCtx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			pieceQueue.Start(queueCtx)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			cancel()
			return nil
		},
	})
	return pieceQueue, nil
}

func RegisterLinkQueueJobs(lq *jobqueue.JobQueue[datamodel.Link], pa *aggregator.PieceAccepter, as *aggregator.AggregateSubmitter) error {
	if err := lq.Register(PieceAcceptTask, func(ctx context.Context, msg datamodel.Link) error {
		return pa.AcceptPieces(ctx, []datamodel.Link{msg})
	}); err != nil {
		return fmt.Errorf("registering %s task: %w", PieceAcceptTask, err)
	}

	if err := lq.Register(PieceSubmitTask, func(ctx context.Context, msg datamodel.Link) error {
		return as.SubmitAggregates(ctx, []datamodel.Link{msg})
	}); err != nil {
		return fmt.Errorf("registering %s task: %w", PieceSubmitTask, err)
	}
	return nil
}

func RegisterPieceQueueJobs(pq *jobqueue.JobQueue[piece.PieceLink], pa *aggregator.PieceAggregator) error {
	if err := pq.Register(PieceAggregateTask, func(ctx context.Context, msg piece.PieceLink) error {
		return pa.AggregatePieces(ctx, []piece.PieceLink{msg})
	}); err != nil {
		return fmt.Errorf("registering %s task: %w", PieceAggregateTask, err)
	}
	return nil
}
