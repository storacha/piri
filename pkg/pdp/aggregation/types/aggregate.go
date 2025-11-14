package types

import (
	// for go:embed
	_ "embed"
	"fmt"

	"github.com/filecoin-project/go-data-segment/merkletree"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/piece/piece"
	"go.uber.org/zap/zapcore"
)

//go:embed aggregate.ipldsch
var aggregateSchema []byte

var aggregateTS *schema.TypeSystem

func init() {
	ts, err := types.LoadSchemaBytes(aggregateSchema)
	if err != nil {
		panic(fmt.Errorf("loading blob schema: %w", err))
	}
	aggregateTS = ts
}

func AggregateType() schema.Type {
	return aggregateTS.TypeByName("Aggregate")
}

func PieceLinkType() schema.Type {
	return aggregateTS.TypeByName("PieceLink")
}

type AggregatePiece struct {
	Link           piece.PieceLink
	InclusionProof merkletree.ProofData
}

type Aggregate struct {
	Root   piece.PieceLink
	Pieces []AggregatePiece
}

// MarshalLogObject makes Aggregate implement the zapcore.ObjectMarshaler interface
func (a Aggregate) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("root", a.Root.Link().String())

	// One approach is to encode the slice as a separate array:
	return enc.AddArray("pieces", zapcore.ArrayMarshalerFunc(func(arr zapcore.ArrayEncoder) error {
		for _, p := range a.Pieces {
			// Log each piece as a string.
			arr.AppendString(p.Link.Link().String())
		}
		return nil
	}))

}
