package aggregator

import (
	"context"
	"fmt"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-libstoracha/piece/piece"
	"github.com/storacha/piri/internal/ipldstore"
	"github.com/storacha/piri/lib/jobqueue"
	"github.com/storacha/piri/pkg/pdp/aggregator/aggregate"
	"go.uber.org/fx"
)

var log = logging.Logger("pdp/aggregator")

const WorkspaceKey = "workspace/"
const AggregatePrefix = "aggregates/"

type AggregatorParams struct {
	fx.In

	Queue       jobqueue.Service[piece.PieceLink]
	TaskHandler TaskHandler[piece.PieceLink]
}

func NewAggregator(lc fx.Lifecycle, params AggregatorParams) (Aggregator, error) {
	if err := params.Queue.Register(AggregatorTaskName, params.TaskHandler.Handle); err != nil {
		return nil, err
	}

	a := &LocalAggregator{
		queue: params.Queue,
	}

	queueCtx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return a.queue.Start(queueCtx)
		},
		OnStop: func(ctx context.Context) error {
			cancel()
			return a.queue.Stop(queueCtx)
		},
	})
	return a, nil
}

// LocalAggregator is a local aggregator running directly on the storage node
// when run w/o cloud infra
type LocalAggregator struct {
	queue jobqueue.Service[piece.PieceLink]
}

// AggregatePiece is the frontend to aggregation
func (la *LocalAggregator) AggregatePiece(ctx context.Context, pieceLink piece.PieceLink) error {
	log.Infow("Aggregating piece", "piece", pieceLink.Link().String())
	return la.queue.Enqueue(ctx, AggregatorTaskName, pieceLink)
}

type StoreParams struct {
	fx.In
	Datastore datastore.Datastore `name:"aggregator_datastore"`
}

func NewAggregatorStore(params StoreParams) AggregateStore {
	return ipldstore.IPLDStore[datamodel.Link, aggregate.Aggregate](
		store.SimpleStoreFromDatastore(namespace.Wrap(params.Datastore, datastore.NewKey(AggregatePrefix))),
		aggregate.AggregateType(), types.Converters...,
	)
}

type AggregatorHandlerOption func(pa *AggregatorHandler)

func WithAggregatorHandler(a BufferedAggregator) AggregatorHandlerOption {
	return func(pa *AggregatorHandler) {
		pa.aggregator = a
	}
}

type AggregatorHandler struct {
	workspace  InProgressWorkspace
	store      AggregateStore
	queue      jobqueue.Service[piece.PieceLink]
	aggregator BufferedAggregator
	Manager    *Manager
}

func NewAggregatorHandler(workspace InProgressWorkspace, store AggregateStore, manager *Manager, queueSubmission jobqueue.Service[piece.PieceLink], opts ...AggregatorHandlerOption) TaskHandler[piece.PieceLink] {
	pa := &AggregatorHandler{
		workspace: workspace,
		store:     store,
		queue:     queueSubmission,
		// default aggregator is BufferingAggregator, it can be overridden via options.
		aggregator: &BufferingAggregator{},
		Manager:    manager,
	}

	for _, opt := range opts {
		opt(pa)
	}
	return pa
}

func (pa *AggregatorHandler) Handle(ctx context.Context, p piece.PieceLink) error {
	buffer, err := pa.workspace.GetBuffer(ctx)
	if err != nil {
		return fmt.Errorf("reading in progress pieces from work space: %w", err)
	}
	buffer, aggregates, err := pa.aggregator.AggregatePieces(buffer, []piece.PieceLink{p})
	if err != nil {
		return fmt.Errorf("calculating aggegates: %w", err)
	}
	if err := pa.workspace.PutBuffer(ctx, buffer); err != nil {
		return fmt.Errorf("updating work space: %w", err)
	}
	for _, a := range aggregates {
		err := pa.store.Put(ctx, a.Root.Link(), a)
		if err != nil {
			return fmt.Errorf("storing aggregate: %w", err)
		}
		if err := pa.Manager.Submit(ctx, a.Root.Link()); err != nil {
			return fmt.Errorf("queueing aggregates for submission: %w", err)
		}
	}
	return nil
}
