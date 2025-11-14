package piece

import (
	"context"
	"errors"
	"fmt"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/blobstore"
)

const DefaultHasSetSize = 100_000

type StoreReader struct {
	store  blobstore.PDPStore
	hasSet mapset.Set[string]
}

func NewStoreReader(store blobstore.PDPStore) (types.PieceReaderAPI, error) {
	return &StoreReader{
		store:  store,
		hasSet: mapset.NewSetWithSize[string](DefaultHasSetSize),
	}, nil
}

func (r *StoreReader) Read(ctx context.Context, blob multihash.Multihash, options ...types.ReadPieceOption) (*types.PieceReader, error) {
	cfg := types.ReadPieceConfig{}
	cfg.ProcessOptions(options)

	var getOptions []blobstore.GetOption
	if cfg.ByteRange.Start > 0 || cfg.ByteRange.End != nil {
		getOptions = append(getOptions, blobstore.WithRange(cfg.ByteRange.Start, cfg.ByteRange.End))
	}

	obj, err := r.store.Get(ctx, blob, getOptions...)
	if err != nil {
		return nil, fmt.Errorf("reading data: %w", err)
	}

	// Note: `Size` must reflect the *total* data size, not just the requested range length.
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

func (r *StoreReader) Has(ctx context.Context, blob multihash.Multihash) (bool, error) {
	if r.hasSet.ContainsOne(blob.String()) {
		return true, nil
	}
	_, err := r.store.Get(ctx, blob)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return false, nil
		}
		return false, types.WrapError(types.KindInternal, "failed to read data", err)
	}
	r.hasSet.Add(blob.String())
	return true, nil
}
