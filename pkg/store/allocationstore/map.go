package allocationstore

import (
	"context"
	"strings"

	multihash "github.com/multiformats/go-multihash"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/allocationstore/allocation"
)

// MapAllocationStore is a store for allocations, backed by an in-memory map.
type MapAllocationStore struct {
	data map[string]allocation.Allocation
}

func (m *MapAllocationStore) Get(ctx context.Context, digest multihash.Multihash, space did.DID) (allocation.Allocation, error) {
	a, ok := m.data[encodeKey(digest, space)]
	if !ok {
		return allocation.Allocation{}, store.ErrNotFound
	}
	return a, nil
}

func (m *MapAllocationStore) List(ctx context.Context, digest multihash.Multihash, options ...ListOption) ([]allocation.Allocation, error) {
	cfg := ListConfig{}
	for _, opt := range options {
		opt(&cfg)
	}

	var allocs []allocation.Allocation
	pfx := encodeKeyPrefix(digest)
	for k, v := range m.data {
		if strings.HasPrefix(k, pfx) {
			allocs = append(allocs, v)
		}
		if cfg.Limit > 0 && len(allocs) == cfg.Limit {
			break
		}
	}

	return allocs, nil
}

func (m *MapAllocationStore) Put(ctx context.Context, alloc allocation.Allocation) error {
	m.data[encodeKey(alloc.Blob.Digest, alloc.Space)] = alloc
	return nil
}

// NewMapAllocationStore creates a for allocations, backed by an in-memory map.
func NewMapAllocationStore() *MapAllocationStore {
	return &MapAllocationStore{data: map[string]allocation.Allocation{}}
}

var _ AllocationStore = (*MapAllocationStore)(nil)
