package aggregator

import (
	"context"
	"fmt"
	"sync"

	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-libstoracha/piece/piece"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/ucan"

	"github.com/storacha/piri/internal/ipldstore"
	"github.com/storacha/piri/pkg/pdp/aggregator/aggregate"
	"github.com/storacha/piri/pkg/pdp/aggregator/fns"
	types2 "github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

type QueuePieceAggregationFn func(context.Context, piece.PieceLink) error

// Step 1: Generate aggregates from pieces

type InProgressWorkspace interface {
	GetBuffer(context.Context) (fns.Buffer, error)
	PutBuffer(context.Context, fns.Buffer) error
}

type bufferKey struct{}

func (bufferKey) String() string { return "buffer" }

type inProgressWorkSpace struct {
	store ipldstore.KVStore[bufferKey, fns.Buffer]
}

func (i *inProgressWorkSpace) GetBuffer(ctx context.Context) (fns.Buffer, error) {
	buf, err := i.store.Get(ctx, bufferKey{})
	if store.IsNotFound(err) {
		err := i.store.Put(ctx, bufferKey{}, fns.Buffer{})
		return fns.Buffer{}, err
	}
	return buf, err
}

func (i *inProgressWorkSpace) PutBuffer(ctx context.Context, buffer fns.Buffer) error {
	return i.store.Put(ctx, bufferKey{}, buffer)
}

func NewInProgressWorkspace(store store.Store) InProgressWorkspace {
	return &inProgressWorkSpace{
		ipldstore.IPLDStore[bufferKey, fns.Buffer](store, fns.BufferType(), types.Converters...),
	}
}

type AggregateStore ipldstore.KVStore[datamodel.Link, aggregate.Aggregate]

type LinkQueue interface {
	Enqueue(ctx context.Context, name string, msg datamodel.Link) error
}

type PieceAggregatorOption func(pa *PieceAggregator)

func WithAggregator(a BufferedAggregator) PieceAggregatorOption {
	return func(pa *PieceAggregator) {
		pa.aggregator = a
	}
}

type PieceAggregator struct {
	workspace  InProgressWorkspace
	store      AggregateStore
	queue      LinkQueue
	aggregator BufferedAggregator
}

func NewPieceAggregator(workspace InProgressWorkspace, store AggregateStore, queueSubmission LinkQueue, opts ...PieceAggregatorOption) *PieceAggregator {
	pa := &PieceAggregator{
		workspace: workspace,
		store:     store,
		queue:     queueSubmission,
		// default aggregator is BufferingAggregator, it can be overridden via options.
		aggregator: &BufferingAggregator{},
	}

	for _, opt := range opts {
		opt(pa)
	}
	return pa
}

func (pa *PieceAggregator) AggregatePieces(ctx context.Context, pieces []piece.PieceLink) error {
	buffer, err := pa.workspace.GetBuffer(ctx)
	if err != nil {
		return fmt.Errorf("reading in progress pieces from work space: %w", err)
	}
	buffer, aggregates, err := pa.aggregator.AggregatePieces(buffer, pieces)
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
		if err := pa.queue.Enqueue(ctx, PieceSubmitTask, a.Root.Link()); err != nil {
			return fmt.Errorf("queueing aggregates for submission: %w", err)
		}
	}
	return nil
}

type AggregateSubmitter struct {
	proofSetProvider types2.ProofSetProvider
	getProofSet      func(context.Context) (uint64, error)
	store            AggregateStore
	client           types2.ProofSetAPI
	queue            LinkQueue
}

func NewAggregateSubmitteer(proofSetProvider types2.ProofSetProvider, store AggregateStore, client types2.ProofSetAPI, queuePieceAccept LinkQueue) *AggregateSubmitter {
	var once sync.Once
	var proofSet uint64
	var proofSetErr error

	// Create a wrapper that ignores context but uses sync.Once
	getProofSet := func(ctx context.Context) (uint64, error) {
		once.Do(func() {
			proofSet, proofSetErr = proofSetProvider.GetOrCreateProofSet(ctx)
		})
		return proofSet, proofSetErr
	}

	return &AggregateSubmitter{
		proofSetProvider: proofSetProvider,
		getProofSet:      getProofSet,
		store:            store,
		client:           client,
		queue:            queuePieceAccept,
	}
}

func (as *AggregateSubmitter) SubmitAggregates(ctx context.Context, aggregateLinks []datamodel.Link) error {
	log.Infow("Submit aggregates", "count", len(aggregateLinks))
	aggregates := make([]aggregate.Aggregate, 0, len(aggregateLinks))
	for _, aggregateLink := range aggregateLinks {
		aggregate, err := as.store.Get(ctx, aggregateLink)
		if err != nil {
			return fmt.Errorf("reading aggregates: %w", err)
		}
		aggregates = append(aggregates, aggregate)
	}
	proofSet, err := as.getProofSet(ctx)
	if err != nil {
		return fmt.Errorf("getting proof set ID: %w", err)
	}

	if err := fns.SubmitAggregates(ctx, as.client, proofSet, aggregates); err != nil {
		return fmt.Errorf("submitting aggregates to Curio: %w", err)
	}
	for _, aggregateLink := range aggregateLinks {
		err := as.queue.Enqueue(ctx, PieceAcceptTask, aggregateLink)
		if err != nil {
			return fmt.Errorf("queuing piece acceptance: %w", err)
		}
	}
	return nil
}

// Step 3: generate receipts for piece accept

type PieceAccepter struct {
	issuer         ucan.Signer
	aggregateStore AggregateStore
	receiptStore   receiptstore.ReceiptStore
}

func NewPieceAccepter(issuer principal.Signer, aggregateStore AggregateStore, receiptStore receiptstore.ReceiptStore) *PieceAccepter {
	return &PieceAccepter{
		issuer:         issuer,
		aggregateStore: aggregateStore,
		receiptStore:   receiptStore,
	}
}

func (pa *PieceAccepter) AcceptPieces(ctx context.Context, aggregateLinks []datamodel.Link) error {
	aggregates := make([]aggregate.Aggregate, 0, len(aggregateLinks))
	for _, aggregateLink := range aggregateLinks {
		aggregate, err := pa.aggregateStore.Get(ctx, aggregateLink)
		if err != nil {
			return fmt.Errorf("reading aggregates: %w", err)
		}
		aggregates = append(aggregates, aggregate)
	}
	// TODO: Should we actually send a piece accept invocation? It seems unneccesary it's all the same machine
	receipts, err := fns.GenerateReceiptsForAggregates(pa.issuer, aggregates)
	if err != nil {
		return fmt.Errorf("generating receipts: %w", err)
	}
	for _, receipt := range receipts {
		if err := pa.receiptStore.Put(ctx, receipt); err != nil {
			return err
		}
	}
	return nil
}

type BufferingAggregator struct{}

func (a *BufferingAggregator) AggregatePiece(buffer fns.Buffer, newPiece piece.PieceLink) (fns.Buffer, *aggregate.Aggregate, error) {
	return fns.AggregatePiece(buffer, newPiece)
}

func (a *BufferingAggregator) AggregatePieces(buffer fns.Buffer, pieces []piece.PieceLink) (fns.Buffer, []aggregate.Aggregate, error) {
	return fns.AggregatePieces(buffer, pieces)
}
