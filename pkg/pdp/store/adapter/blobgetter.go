package adapter

import (
	"context"
	"fmt"
	"io"

	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/piri/pkg/internal/digestutil"
	"github.com/storacha/piri/pkg/pdp/piecefinder"
	"github.com/storacha/piri/pkg/pdp/piecereader"
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
	pieceFinder piecefinder.PieceFinder
	pieceReader piecereader.PieceReader
	blobSizer   BlobSizer
}

func (psa *BlobGetterAdapter) Get(ctx context.Context, digest multihash.Multihash, opts ...blobstore.GetOption) (blobstore.Object, error) {
	size, err := psa.blobSizer.Size(ctx, digest)
	if err != nil {
		return nil, fmt.Errorf("getting size of blob %s: %w", digestutil.Format(digest), err)
	}
	pieceLink, err := psa.pieceFinder.FindPiece(ctx, digest, size)
	if err != nil {
		return nil, fmt.Errorf("finding piece link for %s: %w", digestutil.Format(digest), err)
	}
	res, err := psa.pieceReader.ReadPiece(ctx, pieceLink.Link().(cidlink.Link).Cid)
	if err != nil {
		return nil, fmt.Errorf("reading piece: %w", err)
	}
	return pieceObject{res.Data, res.Size}, nil
}

// NewBlobGetterAdapter creates a new blob getter that allows retrieving from
// piece storage by user hash (typically sha2-256).
func NewBlobGetterAdapter(finder piecefinder.PieceFinder, reader piecereader.PieceReader, sizer BlobSizer) *BlobGetterAdapter {
	return &BlobGetterAdapter{finder, reader, sizer}
}
