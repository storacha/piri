package types

import (
	_ "embed"
	"fmt"

	ipldprime "github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/storacha/go-libstoracha/piece/piece"
)

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
