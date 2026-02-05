package delegationstore

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/objectstore"
	"github.com/storacha/piri/pkg/store/objectstore/minio"
)

const delegationsPrefix = "delegations/"

// minioSimpleStore adapts a MinIO objectstore.Store to the SimpleStore interface
// required by the underlying DelegationStore implementation.
type minioSimpleStore struct {
	store  *minio.Store
	prefix string
}

func (m *minioSimpleStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := m.store.Get(ctx, m.prefix+key)
	if err != nil {
		if errors.Is(err, objectstore.ErrNotExist) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("getting %s from minio: %w", key, err)
	}
	return obj.Body(), nil
}

func (m *minioSimpleStore) Put(ctx context.Context, key string, size uint64, data io.Reader) error {
	return m.store.Put(ctx, m.prefix+key, size, data)
}

// NewMinioDelegationStore creates a DelegationStore backed by a MinIO S3-compatible store.
// Delegations are stored with keys prefixed by "delegations/" in the bucket.
func NewMinioDelegationStore(s *minio.Store) (DelegationStore, error) {
	return NewDelegationStore(&minioSimpleStore{
		store:  s,
		prefix: delegationsPrefix,
	})
}
