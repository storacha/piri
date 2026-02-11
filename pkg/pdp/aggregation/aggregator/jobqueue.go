package aggregator

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"runtime"
	"slices"
	"time"

	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	captypes "github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/piece/piece"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"

	"github.com/storacha/piri/lib/jobqueue"
	"github.com/storacha/piri/lib/jobqueue/dialect"
	"github.com/storacha/piri/lib/jobqueue/serializer"
	"github.com/storacha/piri/lib/jobqueue/traceutil"
	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/pdp/aggregation/manager"
	"github.com/storacha/piri/pkg/pdp/aggregation/types"
)

var log = logging.Logger("aggregation/aggregator")

type AggregatorParams struct {
	fx.In
	Queue   jobqueue.Service[piece.PieceLink]
	Handler jobqueue.TaskHandler[piece.PieceLink]
}

type Aggregator struct {
	queue   jobqueue.Service[piece.PieceLink]
	handler jobqueue.TaskHandler[piece.PieceLink]
}

func New(lc fx.Lifecycle, params AggregatorParams) (*Aggregator, error) {
	a := &Aggregator{
		queue:   params.Queue,
		handler: params.Handler,
	}

	if err := a.queue.RegisterHandler(params.Handler); err != nil {
		return nil, fmt.Errorf("registering aggregator handler: %w", err)
	}

	queueCtx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return a.queue.Start(queueCtx)
		},
		OnStop: func(ctx context.Context) error {
			cancel()
			return a.queue.Stop(ctx)
		},
	})

	return a, nil
}

func (a *Aggregator) EnqueueAggregation(ctx context.Context, piece piece.PieceLink) error {
	log.Infow("enqueuing piece for aggregation", "piece", piece.Link())
	return a.queue.Enqueue(ctx, a.handler.Name(), piece)
}

const (
	QueueName = "aggregator"
	TaskName  = "aggregate_piece"
)

type QueueParams struct {
	fx.In
	DB            *sql.DB `name:"aggregator_db"`
	StorageConfig app.StorageConfig
}

func NewQueue(params QueueParams) (jobqueue.Service[piece.PieceLink], error) {
	// Determine dialect from storage config
	d := dialect.SQLite
	if params.StorageConfig.Database.IsPostgres() {
		d = dialect.Postgres
	}

	// The deduping is required to ensure we don't produce an aggregate with sub roots that exist in another aggregate
	// the behavior here is to ignore duplicate pieces we have already aggregated
	// this is required to ensure roots are added with distinct sub roots from existing roots.
	// While the chain logic permits this, it can lead to duplicate data being proved and thus paid for.
	// Do not allow successfully complete jobs to run again.
	dedupEnabled := true
	// Allow jobs in dead letter queue (failed) to run again.
	blockDLQRetries := false
	linkQueue, err := jobqueue.New[piece.PieceLink](
		QueueName,
		params.DB,
		&serializer.IPLDCBOR[piece.PieceLink]{
			Typ:  types.PieceLinkType(),
			Opts: captypes.Converters,
		},
		jobqueue.WithLogger(log.With("queue", QueueName)),
		jobqueue.WithMaxRetries(50),
		// one worker to keep things serial
		jobqueue.WithMaxWorkers(uint(runtime.NumCPU())),
		// one filecoin epoch since this is wrongly running tasks, we need yet another queue.....
		jobqueue.WithMaxTimeout(30*time.Second),
		jobqueue.WithDialect(d),
		// we enable de-duplication for this queue since we only want to aggregate a piece once.
		jobqueue.WithDedupQueue(&jobqueue.DedupQueueConfig{
			DedupeEnabled:     &dedupEnabled,
			BlockRepeatsOnDLQ: &blockDLQRetries,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("creating aggregator job-queue: %w", err)
	}
	return linkQueue, nil
}

type HandlerParams struct {
	fx.In
	Store     types.Store
	Datastore datastore.Datastore `name:"aggregator_datastore"`
	Manager   *manager.Manager
}

func NewHandler(params HandlerParams) jobqueue.TaskHandler[piece.PieceLink] {
	return &Handler{
		workspace: newInProgressWorkspace(params.Datastore),
		store:     params.Store,
		manager:   params.Manager,
	}
}

type Handler struct {
	workspace InProgressWorkspace
	store     types.Store
	manager   *manager.Manager
}

func (p *Handler) Handle(ctx context.Context, piece piece.PieceLink) (retErr error) {
	ctx, span := traceutil.StartSpan(ctx, tracer, "aggregator.Handle", trace.WithAttributes(attribute.String("piece", piece.Link().String())))
	defer func() {
		if retErr != nil {
			span.RecordError(retErr)
			span.SetStatus(codes.Error, "failed to aggregate piece")
		}
		span.End()
	}()

	log.Infow("aggregating piece", "link", piece.Link())
	buffer, err := p.workspace.GetBuffer(ctx)
	if err != nil {
		return fmt.Errorf("reading in progress pieces from work space: %w", err)
	}
	buffer, a, err := AggregatePiece(buffer, piece)
	if err != nil {
		return fmt.Errorf("calculating aggegates: %w", err)
	}
	if err := p.workspace.PutBuffer(ctx, buffer); err != nil {
		return fmt.Errorf("updating work space: %w", err)
	}
	if a != nil {
		span.AddEvent("aggregate created", trace.WithAttributes(attribute.String("aggregate.root", a.Root.Link().String())))
		if err := p.store.Put(ctx, a.Root.Link(), *a); err != nil {
			return fmt.Errorf("storing aggregate: %w", err)
		}
		if err := p.manager.Submit(ctx, a.Root.Link()); err != nil {
			return fmt.Errorf("submitting aggregate to manager: %w", err)
		}
	}
	return nil
}

func (p *Handler) Name() string {
	return TaskName
}

// MinAggregateSize is 128MB
// Max size is 256MB -- this means we will never see an individual piece larger
// than 256MB -- the upload will fail otherwise
// So we can safely assume that if we see a 256MB piece, we just submit immediately
// If not, we can safely aggregate till >=128MB without going over 256MB
const MinAggregateSize = 128 << 20

func AggregatePiece(buffer types.Buffer, newPiece piece.PieceLink) (types.Buffer, *types.Aggregate, error) {
	log.Infow("aggregating piece",
		"link", newPiece.Link().String(),
		"padded size", newPiece.PaddedSize(),
		"buffer size", buffer.TotalSize,
	)
	// if the piece is aggregatable on its own it should submit immediately
	if newPiece.PaddedSize() > MinAggregateSize {
		aggregate, err := NewAggregate([]piece.PieceLink{newPiece})
		if err == nil {
			log.Infow("aggregate create", "root", aggregate.Root.Link())
		}
		return buffer, &aggregate, err
	}

	newSize := buffer.TotalSize + newPiece.PaddedSize()
	newPieces := InsertOrderedByDescendingSize(buffer.ReverseSortedPieces, newPiece)

	// if we have reached the minimum aggregate size, submit and start over
	if newSize >= MinAggregateSize {
		aggregate, err := NewAggregate(newPieces)
		if err != nil {
			return buffer, nil, err
		}
		log.Infow("aggregate create", "root", aggregate.Root.Link())
		return types.Buffer{}, &aggregate, err
	}

	// otherwise keep aggregating
	return types.Buffer{
		TotalSize:           newSize,
		ReverseSortedPieces: newPieces,
	}, nil, nil
}

func AggregatePieces(buffer types.Buffer, pieces []piece.PieceLink) (types.Buffer, []types.Aggregate, error) {
	var aggregates []types.Aggregate
	for _, piece := range pieces {
		var aggregate *types.Aggregate
		var err error
		buffer, aggregate, err = AggregatePiece(buffer, piece)
		if err != nil {
			return buffer, aggregates, err
		}
		if aggregate != nil {
			aggregates = append(aggregates, *aggregate)
		}
	}
	return buffer, aggregates, nil
}

// InsertOrderedByDescendingSize adds a piece to a list of pieces sorted largest to smallest, maintaining sort order
func InsertOrderedByDescendingSize(sortedPieces []piece.PieceLink, newPiece piece.PieceLink) []piece.PieceLink {
	pos, _ := slices.BinarySearchFunc(sortedPieces, newPiece, func(test, target piece.PieceLink) int {
		// flip ordering comparing size cause we're going in reverse order
		return cmp.Compare(target.PaddedSize(), test.PaddedSize())
	})
	return slices.Insert(sortedPieces, pos, newPiece)
}
