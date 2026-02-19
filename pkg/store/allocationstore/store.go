package allocationstore

import (
	"context"
	"fmt"

	"github.com/ipfs/go-datastore"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/digestutil"
	"github.com/storacha/go-ucanto/did"

	"github.com/storacha/piri/pkg/store/allocationstore/allocation"
	"github.com/storacha/piri/pkg/store/genericstore"
	"github.com/storacha/piri/pkg/store/objectstore"
	"github.com/storacha/piri/pkg/store/objectstore/dsadapter"
	"github.com/storacha/piri/pkg/store/objectstore/minio"
)

// AllocationStore tracks the items that have been, or will soon be stored on
// the storage node.
type AllocationStore interface {
	// Get retrieves an allocation for a blob (digest) in a space (DID). It
	// returns [github.com/storacha/piri/pkg/store.ErrNotFound] if the allocation
	// does not exist.
	Get(context.Context, multihash.Multihash, did.DID) (allocation.Allocation, error)
	// GetAny retrieves any allocation for a blob (digest), regardless of space.
	// Returns [github.com/storacha/piri/pkg/store.ErrNotFound] if no allocation exists.
	GetAny(context.Context, multihash.Multihash) (allocation.Allocation, error)
	// Exists checks if any allocation exists for a blob (digest).
	Exists(context.Context, multihash.Multihash) (bool, error)
	// Put adds or replaces allocation data in the store.
	Put(context.Context, allocation.Allocation) error
}

// KeyEncoder defines how to encode keys for a specific backend.
type KeyEncoder interface {
	EncodeKey(digest multihash.Multihash, space did.DID) string
	EncodeKeyPrefix(digest multihash.Multihash) string
}

// Store implements AllocationStore backed by any ListableStore.
type Store struct {
	store   *genericstore.Store[allocation.Allocation]
	encoder KeyEncoder
}

var _ AllocationStore = (*Store)(nil)

// New creates an AllocationStore with the given backend, prefix, and key encoder.
func New(backend objectstore.ListableStore, prefix string, encoder KeyEncoder) *Store {
	return &Store{
		store:   genericstore.New[allocation.Allocation](backend, prefix, allocation.Codec{}),
		encoder: encoder,
	}
}

func (s *Store) Get(ctx context.Context, digest multihash.Multihash, space did.DID) (allocation.Allocation, error) {
	alloc, err := s.store.Get(ctx, s.encoder.EncodeKey(digest, space))
	if err != nil {
		return allocation.Allocation{}, fmt.Errorf("getting allocation: %w", err)
	}
	return alloc, nil
}

func (s *Store) GetAny(ctx context.Context, digest multihash.Multihash) (allocation.Allocation, error) {
	alloc, err := s.store.GetAny(ctx, s.encoder.EncodeKeyPrefix(digest))
	if err != nil {
		return allocation.Allocation{}, fmt.Errorf("getting any allocation: %w", err)
	}
	return alloc, nil
}

func (s *Store) Exists(ctx context.Context, digest multihash.Multihash) (bool, error) {
	return s.store.ExistsWithPrefix(ctx, s.encoder.EncodeKeyPrefix(digest))
}

func (s *Store) Put(ctx context.Context, alloc allocation.Allocation) error {
	return s.store.Put(ctx, s.encoder.EncodeKey(alloc.Blob.Digest, alloc.Space), alloc)
}

// S3KeyEncoder encodes keys for S3/MinIO backends (keys end with .cbor).
type S3KeyEncoder struct{}

func (S3KeyEncoder) EncodeKey(digest multihash.Multihash, space did.DID) string {
	return fmt.Sprintf("%s/%s.cbor", digestutil.Format(digest), space.String())
}

func (S3KeyEncoder) EncodeKeyPrefix(digest multihash.Multihash) string {
	return fmt.Sprintf("%s/", digestutil.Format(digest))
}

// DatastoreKeyEncoder encodes keys for LevelDB/datastore backends (no suffix).
type DatastoreKeyEncoder struct{}

func (DatastoreKeyEncoder) EncodeKey(digest multihash.Multihash, space did.DID) string {
	return fmt.Sprintf("%s/%s", digestutil.Format(digest), space.String())
}

func (DatastoreKeyEncoder) EncodeKeyPrefix(digest multihash.Multihash) string {
	return fmt.Sprintf("%s/", digestutil.Format(digest))
}

// NewS3Store creates an AllocationStore for S3/MinIO backends.
// Allocations are stored with keys formatted as "allocations/{digest}/{space}.cbor".
func NewS3Store(backend *minio.Store) *Store {
	return New(backend, "allocations/", S3KeyEncoder{})
}

// NewDatastoreStore creates an AllocationStore for LevelDB/datastore backends.
// Allocations are stored with keys formatted as "{digest}/{space}".
func NewDatastoreStore(ds datastore.Datastore) *Store {
	return New(dsadapter.New(ds), "", DatastoreKeyEncoder{})
}
