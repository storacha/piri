package adapter

import (
	"context"
	"fmt"
	"io"

	"github.com/multiformats/go-multihash"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store/blobstore"
)

type pieceObject struct {
	body io.ReadCloser
	size int64
}

func (o pieceObject) Size() int64 {
	return o.size
}

func (o pieceObject) Body() io.ReadCloser {
	return o.body
}

type BlobSizer interface {
	// Size returns the total size of the blob identified by the given hash.
	Size(context.Context, multihash.Multihash) (uint64, error)
}

// BlobGetterAdapter adapts a PDP piece finder and piece reader into a
// [blobstore.BlobGetter]
type BlobGetterAdapter struct {
	pieceReader types.PieceReaderAPI
}

func (bga *BlobGetterAdapter) Get(ctx context.Context, digest multihash.Multihash, opts ...blobstore.GetOption) (blobstore.Object, error) {
	cfg := blobstore.GetOptions{}
	cfg.ProcessOptions(opts)

	var readOptions []types.ReadPieceOption
	if cfg.ByteRange.Start > 0 || cfg.ByteRange.End != nil {
		readOptions = append(readOptions, types.WithRange(cfg.ByteRange.Start, cfg.ByteRange.End))
	}
	res, err := bga.pieceReader.Read(ctx, digest, readOptions...)
	if err != nil {
		return nil, fmt.Errorf("reading piece: %w", err)
	}
	return pieceObject{res.Data, res.Size}, nil
}

// NewBlobGetterAdapter creates a new blob getter that allows retrieving from
// piece storage by user hash (typically sha2-256).
func NewBlobGetterAdapter(reader types.PieceReaderAPI) *BlobGetterAdapter {
	return &BlobGetterAdapter{reader}
}
