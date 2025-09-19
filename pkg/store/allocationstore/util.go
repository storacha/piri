package allocationstore

import (
	"context"
	"fmt"

	multihash "github.com/multiformats/go-multihash"
	"github.com/storacha/piri/pkg/store"
)

// BlobSizer is a utility to obtain the size of a blob using data from the
// allocation table.
type BlobSizer struct {
	store AllocationStore
}

// NewBlobSizer creates a new utility to obtain the size of a blob using data
// from the allocation table.
func NewBlobSizer(store AllocationStore) *BlobSizer {
	return &BlobSizer{store}
}

// Size returns the size of the blob, provided it has been allocated. Returns
// [store.ErrNotFound] if there are no allocations in the store the for blob.
func (bs *BlobSizer) Size(ctx context.Context, digest multihash.Multihash) (uint64, error) {
	allocs, err := bs.store.List(ctx, digest, WithLimit(1))
	if err != nil {
		return 0, fmt.Errorf("listing allocations: %w", err)
	}
	if len(allocs) == 0 {
		return 0, store.ErrNotFound
	}
	return allocs[0].Blob.Size, nil
}
