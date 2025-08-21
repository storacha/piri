package aggregator

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-libstoracha/piece/piece"
	"github.com/storacha/go-ucanto/principal"

	"github.com/storacha/piri/internal/ipldstore"
	"github.com/storacha/piri/pkg/database"
	"github.com/storacha/piri/pkg/database/sqlitedb"
	"github.com/storacha/piri/pkg/pdp/aggregator/aggregate"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue"
	types2 "github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

var log = logging.Logger("pdp/aggregator")

const WorkspaceKey = "workspace/"
const AggregatePrefix = "aggregates/"

const (
	LinkQueueName  = "link"
	PieceQueueName = "piece"
)

// task names
const (
	PieceAggregateTask = "piece_aggregate"
	PieceSubmitTask    = "piece_submit"
	PieceAcceptTask    = "piece_accept"
)

// LocalAggregator is a local aggregator running directly on the storage node
// when run w/o cloud infra
type LocalAggregator struct {
	pieceQueue *jobqueue.JobQueue[piece.PieceLink]
	linkQueue  *jobqueue.JobQueue[datamodel.Link]
}

// Startup starts up aggregation queues
func (la *LocalAggregator) Startup(ctx context.Context) error {
	go la.pieceQueue.Start(ctx)
	go la.linkQueue.Start(ctx)
	return nil
}

// Shutdown shuts down aggregation queues
func (la *LocalAggregator) Shutdown(ctx context.Context) {
}

// AggregatePiece is the frontend to aggregation
func (la *LocalAggregator) AggregatePiece(ctx context.Context, pieceLink piece.PieceLink) error {
	log.Infow("Aggregating piece", "piece", pieceLink.Link().String())
	return la.pieceQueue.Enqueue(ctx, PieceAggregateTask, pieceLink)
}

func NewLocalAggregator(pieceQueue *jobqueue.JobQueue[piece.PieceLink], linkQueue *jobqueue.JobQueue[datamodel.Link]) *LocalAggregator {
	return &LocalAggregator{
		pieceQueue: pieceQueue,
		linkQueue:  linkQueue,
	}
}

// NewLocal constructs an aggregator to run directly on a machine from a local datastore
func NewLocal(
	ds datastore.Datastore,
	dbPath string,
	client types2.ProofSetAPI,
	proofSet uint64,
	issuer principal.Signer,
	receiptStore receiptstore.ReceiptStore,
) (*LocalAggregator, error) {
	aggregateStore := ipldstore.IPLDStore[datamodel.Link, aggregate.Aggregate](
		store.SimpleStoreFromDatastore(namespace.Wrap(ds, datastore.NewKey(AggregatePrefix))),
		aggregate.AggregateType(), types.Converters...)
	inProgressWorkspace := NewInProgressWorkspace(store.SimpleStoreFromDatastore(namespace.Wrap(ds, datastore.NewKey(WorkspaceKey))))

	db, err := sqlitedb.New(dbPath,
		database.WithJournalMode("WAL"),
		database.WithTimeout(5*time.Second),
		database.WithSyncMode(database.SyncModeNORMAL),
	)
	if err != nil {
		return nil, fmt.Errorf("creating jobqueue database: %w", err)
	}
	linkQueue, err := NewLinkQueue(db)
	if err != nil {
		return nil, err
	}

	pieceQueue, err := NewPieceQueue(db)
	if err != nil {
		return nil, err
	}

	// construct queues -- somewhat frstratingly these have to be constructed backward for now
	pieceAccepter := NewPieceAccepter(issuer, aggregateStore, receiptStore)
	aggregationSubmitter := NewAggregateSubmitter(&ConfiguredProofSetProvider{ID: proofSet}, aggregateStore, client, linkQueue)
	pieceAggregator := NewPieceAggregator(inProgressWorkspace, aggregateStore, linkQueue)

	if err := linkQueue.Register(PieceAcceptTask, func(ctx context.Context, msg datamodel.Link) error {
		return pieceAccepter.AcceptPieces(ctx, []datamodel.Link{msg})
	}); err != nil {
		return nil, fmt.Errorf("registering %s task: %w", PieceAcceptTask, err)
	}

	if err := linkQueue.Register(PieceSubmitTask, func(ctx context.Context, msg datamodel.Link) error {
		return aggregationSubmitter.SubmitAggregates(ctx, []datamodel.Link{msg})
	}); err != nil {
		return nil, fmt.Errorf("registering %s task: %w", PieceSubmitTask, err)
	}

	if err := pieceQueue.Register(PieceAggregateTask, func(ctx context.Context, msg piece.PieceLink) error {
		return pieceAggregator.AggregatePieces(ctx, []piece.PieceLink{msg})
	}); err != nil {
		return nil, fmt.Errorf("registering %s task: %w", PieceAggregateTask, err)
	}

	return &LocalAggregator{
		pieceQueue: pieceQueue,
		linkQueue:  linkQueue,
	}, nil
}
