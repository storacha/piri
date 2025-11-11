package aggregator

import (
	"context"

	"github.com/storacha/go-libstoracha/piece/piece"

	"github.com/storacha/piri/pkg/pdp/aggregator/aggregate"
	"github.com/storacha/piri/pkg/pdp/aggregator/fns"
)

// Aggregator is an interface for accessing an aggregation queue
type Aggregator interface {
	AggregatePiece(ctx context.Context, pieceLink piece.PieceLink) error
}

type BufferedAggregator interface {
	AggregatePiece(buffer fns.Buffer, newPiece piece.PieceLink) (fns.Buffer, *aggregate.Aggregate, error)
	AggregatePieces(buffer fns.Buffer, pieces []piece.PieceLink) (fns.Buffer, []aggregate.Aggregate, error)
}

type ProofSetIDProvider interface {
	ProofSetID(ctx context.Context) (uint64, error)
}

type TaskHandler[T any] interface {
	Handle(ctx context.Context, data T) error
}
