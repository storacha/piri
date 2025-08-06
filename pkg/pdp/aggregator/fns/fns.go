package fns

import (
	"cmp"
	"context"
	// for go:embed
	_ "embed"
	"fmt"
	"slices"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	ipldprime "github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/storacha/go-libstoracha/capabilities/pdp"
	"github.com/storacha/go-libstoracha/piece/piece"
	"github.com/storacha/go-ucanto/core/invocation/ran"
	"github.com/storacha/go-ucanto/core/ipld"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/ucan"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/storacha/piri/pkg/pdp/aggregator/aggregate"
	types2 "github.com/storacha/piri/pkg/pdp/types"
)

var log = logging.Logger("fns")

//go:embed buffer.ipldsch
var bufferSchema []byte

var bufferTS *schema.TypeSystem

func init() {
	ts, err := ipldprime.LoadSchemaBytes(bufferSchema)
	if err != nil {
		panic(fmt.Errorf("loading buffer schema: %w", err))
	}
	bufferTS = ts
}

func BufferType() schema.Type {
	return bufferTS.TypeByName("Buffer")
}

// Buffer tracks in progress work building an aggregation
type Buffer struct {
	TotalSize           uint64
	ReverseSortedPieces []piece.PieceLink
}

// InsertOrderedByDescendingSize adds a piece to a list of pieces sorted largest to smallest, maintaining sort order
func InsertOrderedByDescendingSize(sortedPieces []piece.PieceLink, newPiece piece.PieceLink) []piece.PieceLink {
	pos, _ := slices.BinarySearchFunc(sortedPieces, newPiece, func(test, target piece.PieceLink) int {
		// flip ordering comparing size cause we're going in reverse order
		return cmp.Compare(target.PaddedSize(), test.PaddedSize())
	})
	return slices.Insert(sortedPieces, pos, newPiece)
}

// MinAggregateSize is 128MB
// Max size is 256MB -- this means we will never see an individual piece larger
// than 256MB -- the upload will fail otherwise
// So we can safely assume that if we see a 256MB piece, we just submit immediately
// If not, we can safely aggregate till >=128MB without going over 256MB
const MinAggregateSize = 128 << 20

func AggregatePiece(buffer Buffer, newPiece piece.PieceLink) (Buffer, *aggregate.Aggregate, error) {
	log.Infow("Aggregate Piece",
		"link", newPiece.Link().String(),
		"padded size", newPiece.PaddedSize(),
		"buffer size", buffer.TotalSize,
	)
	// if the piece is aggregatable on its own it should submit immediately
	if newPiece.PaddedSize() > MinAggregateSize {
		aggregate, err := aggregate.NewAggregate([]piece.PieceLink{newPiece})
		return buffer, &aggregate, err
	}

	newSize := buffer.TotalSize + newPiece.PaddedSize()
	newPieces := InsertOrderedByDescendingSize(buffer.ReverseSortedPieces, newPiece)

	// if we have reached the minimum aggregate size, submit and start over
	if newSize >= MinAggregateSize {
		aggregate, err := aggregate.NewAggregate(newPieces)
		if err != nil {
			return buffer, nil, err
		}
		return Buffer{}, &aggregate, err
	}

	// otherwise keep aggregating
	return Buffer{
		TotalSize:           newSize,
		ReverseSortedPieces: newPieces,
	}, nil, nil
}

func AggregatePieces(buffer Buffer, pieces []piece.PieceLink) (Buffer, []aggregate.Aggregate, error) {
	var aggregates []aggregate.Aggregate
	for _, piece := range pieces {
		var aggregate *aggregate.Aggregate
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

func SubmitAggregates(ctx context.Context, client types2.ProofSetAPI, proofSet uint64, aggregates []aggregate.Aggregate) error {
	log.Info("submit aggregates",
		zap.Array("aggregates", zapcore.ArrayMarshalerFunc(func(arr zapcore.ArrayEncoder) error {
			for _, agg := range aggregates { // aggregates is []Aggregate
				arr.AppendObject(agg) // agg already implements ObjectMarshaler
			}
			return nil
		})),
	)
	newRoots := make([]types2.RootAdd, 0, len(aggregates))
	for _, a := range aggregates {
		rootCID, err := cid.Decode(a.Root.V1Link().String())
		if err != nil {
			return fmt.Errorf("failed to decode aggregate root CID: %w", err)
		}
		subRoots := make([]cid.Cid, 0, len(a.Pieces))
		for _, p := range a.Pieces {
			pcid, err := cid.Decode(p.Link.V1Link().String())
			if err != nil {
				return fmt.Errorf("failed to decode piece CID: %w", err)
			}
			subRoots = append(subRoots, pcid)
		}
		newRoots = append(newRoots, types2.RootAdd{
			Root:     rootCID,
			SubRoots: subRoots,
		})
	}
	_, err := client.AddRoots(ctx, proofSet, newRoots)
	if err != nil {
		return fmt.Errorf("failed to submit aggregates: %w", err)
	}
	return nil
}

func GenerateReceipts(issuer ucan.Signer, aggregate aggregate.Aggregate) ([]receipt.AnyReceipt, error) {
	receipts := make([]receipt.AnyReceipt, 0, len(aggregate.Pieces))
	for _, aggregatePiece := range aggregate.Pieces {
		inv, err := pdp.Accept.Invoke(issuer, issuer, issuer.DID().String(), pdp.AcceptCaveats{
			Piece: aggregatePiece.Link,
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

func GenerateReceiptsForAggregates(issuer ucan.Signer, aggregates []aggregate.Aggregate) ([]receipt.AnyReceipt, error) {
	size := 0
	for _, aggregate := range aggregates {
		size += len(aggregate.Pieces)
	}
	receipts := make([]receipt.AnyReceipt, 0, size)
	for _, aggregate := range aggregates {
		aggregateReceipts, err := GenerateReceipts(issuer, aggregate)
		if err != nil {
			return nil, err
		}
		receipts = append(receipts, aggregateReceipts...)
	}
	return receipts, nil
}
