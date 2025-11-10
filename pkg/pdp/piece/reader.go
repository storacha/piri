package piece

import (
	"context"
	"fmt"

	"github.com/multiformats/go-multihash"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store/blobstore"
)

type StoreReader struct {
	store blobstore.PDPStore
}

func NewStoreReader(store blobstore.PDPStore) types.PieceReaderAPI {
	return &StoreReader{
		store: store,
	}
}

func (r *StoreReader) ReadPiece(ctx context.Context, piece multihash.Multihash, options ...types.ReadPieceOption) (*types.PieceReader, error) {
	cfg := types.ReadPieceConfig{}
	cfg.ProcessOptions(options)

	resolved := piece
	if resolver := cfg.Resolver; resolver != nil {
		var (
			err   error
			found bool
		)
		resolved, found, err = resolver.ResolvePiece(ctx, piece)
		if err != nil {
			return nil, fmt.Errorf("resolving piece: %w", err)
		}
		if !found {
			return nil, fmt.Errorf("piece %s not found", piece)
		}
	}

	var getOptions []blobstore.GetOption
	if cfg.ByteRange.Start > 0 || cfg.ByteRange.End != nil {
		getOptions = append(getOptions, blobstore.WithRange(cfg.ByteRange.Start, cfg.ByteRange.End))
	}

	obj, err := r.store.Get(ctx, resolved, getOptions...)
	if err != nil {
		return nil, fmt.Errorf("reading piece: %w", err)
	}

	// Note: `Size` must reflect the *total* piece size, not just the requested range length.
	// Browser and CDN caching depend on knowing the full entity size for correct handling of
	// HTTP 206 Partial Content responses (Content-Range: bytes start-end/totalSize).
	// If we report only the slice length, caches can’t reassemble or resume downloads correctly,
	// ETags become inconsistent, and progress reporting breaks.
	// The range length (end-start+1) should be tracked separately if needed for Content-Length.
	return &types.PieceReader{
		Size: obj.Size(),
		Data: obj.Body(),
	}, nil
}
