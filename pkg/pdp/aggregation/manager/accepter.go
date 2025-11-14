package manager

import (
	"context"
	"fmt"

	"github.com/ipld/go-ipld-prime/datamodel"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/storacha/go-libstoracha/capabilities/pdp"
	"github.com/storacha/go-ucanto/core/ipld"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/core/receipt/ran"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/ucan"

	"github.com/storacha/piri/internal/ipldstore"
	"github.com/storacha/piri/pkg/pdp/aggregation/types"
	apitypes "github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

type PieceAccepter struct {
	issuer         principal.Signer
	aggregateStore ipldstore.KVStore[datamodel.Link, types.Aggregate]
	receiptStore   receiptstore.ReceiptStore
	resolver       apitypes.PieceResolverAPI
}

func NewPieceAccepter(issuer principal.Signer, aggregateStore types.Store, receiptStore receiptstore.ReceiptStore, resolver apitypes.PieceResolverAPI) *PieceAccepter {
	return &PieceAccepter{
		issuer:         issuer,
		aggregateStore: aggregateStore,
		receiptStore:   receiptStore,
		resolver:       resolver,
	}
}

func (pa *PieceAccepter) AcceptPieces(ctx context.Context, aggregateLinks []datamodel.Link) error {
	// TODO we can run this in parallel since receipt generation requires resolving pdp pieces in links to blobs
	aggregates := make([]types.Aggregate, 0, len(aggregateLinks))
	for _, aggregateLink := range aggregateLinks {
		aggregate, err := pa.aggregateStore.Get(ctx, aggregateLink)
		if err != nil {
			return fmt.Errorf("reading aggregates: %w", err)
		}
		aggregates = append(aggregates, aggregate)
	}
	// TODO: Should we actually send a piece accept invocation? It seems unnecessary it's all the same machine
	receipts, err := GenerateReceiptsForAggregates(ctx, pa.issuer, aggregates, pa.resolver)
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

func GenerateReceipts(ctx context.Context, issuer ucan.Signer, aggregate types.Aggregate, resolver apitypes.PieceResolverAPI) ([]receipt.AnyReceipt, error) {
	receipts := make([]receipt.AnyReceipt, 0, len(aggregate.Pieces))
	for _, aggregatePiece := range aggregate.Pieces {
		blob, found, err := resolver.ResolveToBlob(ctx, aggregatePiece.Link.Link().(cidlink.Link).Cid.Hash())
		if err != nil {
			return nil, fmt.Errorf("resolving piece for receipt: %w", err)
		}
		if !found {
			return nil, fmt.Errorf("piece not found for receipt generation: %s", aggregatePiece.Link.Link().String())
		}
		inv, err := pdp.Accept.Invoke(issuer, issuer, issuer.DID().String(), pdp.AcceptCaveats{
			Blob: blob,
		})

		if err != nil {
			return nil, fmt.Errorf("generating invocation: %w", err)
		}
		ok := result.Ok[pdp.AcceptOk, ipld.Builder](pdp.AcceptOk{
			Aggregate:      aggregate.Root,
			InclusionProof: aggregatePiece.InclusionProof,
			Piece:          aggregatePiece.Link,
		})
		rcpt, err := receipt.Issue(issuer, ok, ran.FromInvocation(inv))
		if err != nil {
			return nil, fmt.Errorf("issuing receipt: %w", err)
		}
		receipts = append(receipts, rcpt)
	}
	return receipts, nil
}

func GenerateReceiptsForAggregates(ctx context.Context, issuer ucan.Signer, aggregates []types.Aggregate, resolver apitypes.PieceResolverAPI) ([]receipt.AnyReceipt, error) {
	size := 0
	for _, aggregate := range aggregates {
		size += len(aggregate.Pieces)
	}
	receipts := make([]receipt.AnyReceipt, 0, size)
	for _, aggregate := range aggregates {
		aggregateReceipts, err := GenerateReceipts(ctx, issuer, aggregate, resolver)
		if err != nil {
			return nil, err
		}
		receipts = append(receipts, aggregateReceipts...)
	}
	return receipts, nil
}
