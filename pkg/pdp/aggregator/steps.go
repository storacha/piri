package aggregator

import (
	"context"
	"fmt"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/ipld/go-ipld-prime/datamodel"
	captypes "github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-libstoracha/piece/piece"
	"github.com/storacha/go-ucanto/principal"
	"go.uber.org/fx"

	"github.com/storacha/piri/internal/ipldstore"
	"github.com/storacha/piri/pkg/pdp/aggregator/aggregate"
	"github.com/storacha/piri/pkg/pdp/aggregator/fns"
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

type InProgressWorkspaceParams struct {
	fx.In
	Datastore datastore.Datastore `name:"aggregator_datastore"`
}

func NewInProgressWorkspace(params InProgressWorkspaceParams) InProgressWorkspace {
	return &inProgressWorkSpace{
		ipldstore.IPLDStore[bufferKey, fns.Buffer](store.SimpleStoreFromDatastore(namespace.Wrap(params.Datastore, datastore.NewKey(WorkspaceKey))), fns.BufferType(), captypes.Converters...),
	}
}

type AggregateStore ipldstore.KVStore[datamodel.Link, aggregate.Aggregate]

type LinkQueue interface {
	Enqueue(ctx context.Context, name string, msg datamodel.Link) error
}

// Step 3: generate receipts for piece accept

type PieceAccepter struct {
	issuer         principal.Signer
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
	// TODO: Should we actually send a piece accept invocation? It seems unnecessary it's all the same machine
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

type ConfiguredProofSetProvider struct {
	ID uint64
}

func (c *ConfiguredProofSetProvider) ProofSetID(ctx context.Context) (uint64, error) {
	return c.ID, nil
}
