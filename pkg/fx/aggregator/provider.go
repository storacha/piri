package aggregator

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-libstoracha/piece/piece"
	"go.uber.org/fx"

	"github.com/storacha/piri/internal/ipldstore"
	"github.com/storacha/piri/pkg/pdp/aggregator"
	"github.com/storacha/piri/pkg/pdp/aggregator/aggregate"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue"
)

var Module = fx.Module("aggregator",
	fx.Provide(
		fx.Annotate(
			aggregator.NewLocalAggregator,
			fx.As(new(aggregator.Aggregator)),
		),
		aggregator.NewPieceAccepter,
		aggregator.NewAggregateSubmitter,
		aggregator.NewPieceAggregator,
		fx.Annotate(
			ProvideStore,
			fx.As(new(aggregator.AggregateStore)),
		),
		ProvideInProgressWorkspace,
		ProvidePieceQueue,
		fx.Annotate(
			ProvideLinkQueue,
			fx.As(fx.Self()),
			fx.As(new(aggregator.LinkQueue)),
		),
	),

	fx.Invoke(
		RegisterPieceQueueJobs,
		RegisterLinkQueueJobs,
	),
)

type StoreParams struct {
	fx.In
	Datastore datastore.Datastore `name:"aggregator_datastore"`
}

func ProvideStore(params StoreParams) ipldstore.KVStore[datamodel.Link, aggregate.Aggregate] {
	return ipldstore.IPLDStore[datamodel.Link, aggregate.Aggregate](
		store.SimpleStoreFromDatastore(namespace.Wrap(params.Datastore, datastore.NewKey(aggregator.AggregatePrefix))),
		aggregate.AggregateType(), types.Converters...,
	)
}

func ProvideInProgressWorkspace(params StoreParams) aggregator.InProgressWorkspace {
	return aggregator.NewInProgressWorkspace(store.SimpleStoreFromDatastore(namespace.Wrap(params.Datastore, datastore.NewKey(aggregator.WorkspaceKey))))
}

type LinkQueueParams struct {
	fx.In
	DB *sql.DB `name:"aggregator_db"`
}

func ProvideLinkQueue(lc fx.Lifecycle, params LinkQueueParams) (*jobqueue.JobQueue[datamodel.Link], error) {
	linkQueue, err := aggregator.NewLinkQueue(params.DB)
	if err != nil {
		return nil, err
	}
	queueCtx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			linkQueue.Start(queueCtx)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			cancel()
			return linkQueue.Stop(ctx)
		},
	})
	return linkQueue, nil
}

func ProvidePieceQueue(lc fx.Lifecycle, params LinkQueueParams) (*jobqueue.JobQueue[piece.PieceLink], error) {
	pieceQueue, err := aggregator.NewPieceQueue(params.DB)
	if err != nil {
		return nil, err
	}
	queueCtx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			pieceQueue.Start(queueCtx)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			cancel()
			return pieceQueue.Stop(ctx)
		},
	})
	return pieceQueue, nil
}

func RegisterLinkQueueJobs(lq *jobqueue.JobQueue[datamodel.Link], pa *aggregator.PieceAccepter, as *aggregator.AggregateSubmitter) error {
	if err := lq.Register(aggregator.PieceAcceptTask, func(ctx context.Context, msg datamodel.Link) error {
		return pa.AcceptPieces(ctx, []datamodel.Link{msg})
	}); err != nil {
		return fmt.Errorf("registering %s task: %w", aggregator.PieceAcceptTask, err)
	}

	if err := lq.Register(aggregator.PieceSubmitTask, func(ctx context.Context, msg datamodel.Link) error {
		return as.SubmitAggregates(ctx, []datamodel.Link{msg})
	}); err != nil {
		return fmt.Errorf("registering %s task: %w", aggregator.PieceSubmitTask, err)
	}
	return nil
}

func RegisterPieceQueueJobs(pq *jobqueue.JobQueue[piece.PieceLink], pa *aggregator.PieceAggregator) error {
	if err := pq.Register(aggregator.PieceAggregateTask, func(ctx context.Context, msg piece.PieceLink) error {
		return pa.AggregatePieces(ctx, []piece.PieceLink{msg})
	}); err != nil {
		return fmt.Errorf("registering %s task: %w", aggregator.PieceAggregateTask, err)
	}
	return nil
}
