package allocationstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/digestutil"
	"github.com/storacha/go-ucanto/did"

	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/allocationstore/allocation"
	"github.com/storacha/piri/pkg/store/objectstore"
	"github.com/storacha/piri/pkg/store/objectstore/minio"
)

const allocationsPrefix = "allocations/"

// MinioAllocationStore implements AllocationStore using a MinIO S3-compatible backend.
type MinioAllocationStore struct {
	store  *minio.Store
	prefix string
}

var _ AllocationStore = (*MinioAllocationStore)(nil)

// NewMinioAllocationStore creates an AllocationStore backed by a MinIO S3-compatible store.
// Allocations are stored with keys formatted as "allocations/{digest}/{space}.cbor".
func NewMinioAllocationStore(s *minio.Store) *MinioAllocationStore {
	return &MinioAllocationStore{
		store:  s,
		prefix: allocationsPrefix,
	}
}

func (m *MinioAllocationStore) Get(ctx context.Context, digest multihash.Multihash, space did.DID) (allocation.Allocation, error) {
	key := m.encodeKey(digest, space)
	obj, err := m.store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, objectstore.ErrNotExist) {
			return allocation.Allocation{}, store.ErrNotFound
		}
		return allocation.Allocation{}, fmt.Errorf("getting allocation from minio: %w", err)
	}
	defer obj.Body().Close()

	data, err := io.ReadAll(obj.Body())
	if err != nil {
		return allocation.Allocation{}, fmt.Errorf("reading allocation data: %w", err)
	}

	return allocation.Decode(data, dagcbor.Decode)
}

func (m *MinioAllocationStore) List(ctx context.Context, digest multihash.Multihash, options ...ListOption) ([]allocation.Allocation, error) {
	cfg := ListConfig{}
	for _, opt := range options {
		opt(&cfg)
	}

	prefix := m.encodeKeyPrefix(digest)
	var allocs []allocation.Allocation
	count := 0

	for key, err := range m.store.ListPrefix(ctx, prefix) {
		if err != nil {
			return nil, fmt.Errorf("listing allocations: %w", err)
		}

		// Apply limit if configured
		if cfg.Limit > 0 && count >= cfg.Limit {
			break
		}

		obj, err := m.store.Get(ctx, key)
		if err != nil {
			// Skip objects that may have been deleted between list and get
			if errors.Is(err, objectstore.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("getting allocation %s: %w", key, err)
		}

		data, err := io.ReadAll(obj.Body())
		obj.Body().Close()
		if err != nil {
			return nil, fmt.Errorf("reading allocation data: %w", err)
		}

		alloc, err := allocation.Decode(data, dagcbor.Decode)
		if err != nil {
			return nil, fmt.Errorf("decoding allocation: %w", err)
		}

		allocs = append(allocs, alloc)
		count++
	}

	return allocs, nil
}

func (m *MinioAllocationStore) Put(ctx context.Context, alloc allocation.Allocation) error {
	key := m.encodeKey(alloc.Blob.Digest, alloc.Space)

	data, err := allocation.Encode(alloc, dagcbor.Encode)
	if err != nil {
		return fmt.Errorf("encoding allocation: %w", err)
	}

	err = m.store.Put(ctx, key, uint64(len(data)), bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("writing allocation to minio: %w", err)
	}

	return nil
}

// encodeKey creates the S3 object key for an allocation.
// Format: {prefix}{digest}/{space}.cbor
func (m *MinioAllocationStore) encodeKey(digest multihash.Multihash, space did.DID) string {
	return fmt.Sprintf("%s%s/%s.cbor", m.prefix, digestutil.Format(digest), space.String())
}

// encodeKeyPrefix creates the S3 prefix for listing allocations by digest.
// Format: {prefix}{digest}/
func (m *MinioAllocationStore) encodeKeyPrefix(digest multihash.Multihash) string {
	return fmt.Sprintf("%s%s/", m.prefix, digestutil.Format(digest))
}
