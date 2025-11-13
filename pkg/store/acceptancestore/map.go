package acceptancestore

import (
	"context"
	"strings"
	"sync"

	multihash "github.com/multiformats/go-multihash"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/acceptancestore/acceptance"
)

// MapAcceptanceStore is a store for allocations, backed by an in-memory map.
type MapAcceptanceStore struct {
	mutex sync.RWMutex
	data  map[string]acceptance.Acceptance
}

func (m *MapAcceptanceStore) Get(ctx context.Context, digest multihash.Multihash, space did.DID) (acceptance.Acceptance, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	a, ok := m.data[encodeKey(digest, space)]
	if !ok {
		return acceptance.Acceptance{}, store.ErrNotFound
	}
	return a, nil
}

func (m *MapAcceptanceStore) List(ctx context.Context, digest multihash.Multihash, options ...ListOption) ([]acceptance.Acceptance, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	cfg := ListConfig{}
	for _, opt := range options {
		opt(&cfg)
	}

	var allocs []acceptance.Acceptance
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

func (m *MapAcceptanceStore) Put(ctx context.Context, acc acceptance.Acceptance) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.data[encodeKey(acc.Blob.Digest, acc.Space)] = acc
	return nil
}

// NewMapAcceptanceStore creates a for allocations, backed by an in-memory map.
func NewMapAcceptanceStore() *MapAcceptanceStore {
	return &MapAcceptanceStore{data: map[string]acceptance.Acceptance{}}
}

var _ AcceptanceStore = (*MapAcceptanceStore)(nil)
