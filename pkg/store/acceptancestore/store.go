package acceptancestore

import (
	"context"
	"fmt"

	"github.com/ipfs/go-datastore"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/digestutil"
	"github.com/storacha/go-ucanto/did"

	"github.com/storacha/piri/pkg/store/acceptancestore/acceptance"
	"github.com/storacha/piri/pkg/store/genericstore"
	"github.com/storacha/piri/pkg/store/objectstore"
	"github.com/storacha/piri/pkg/store/objectstore/dsadapter"
	"github.com/storacha/piri/pkg/store/objectstore/minio"
)

// AcceptanceStore tracks the items that have been stored on the storage node.
type AcceptanceStore interface {
	// Get retrieves an acceptance for a blob (digest) in a space (DID). It
	// returns [github.com/storacha/piri/pkg/store.ErrNotFound] if the acceptance
	// does not exist.
	Get(context.Context, multihash.Multihash, did.DID) (acceptance.Acceptance, error)
	// GetAny retrieves any acceptance for a blob (digest), regardless of space.
	// Returns [github.com/storacha/piri/pkg/store.ErrNotFound] if no acceptance exists.
	GetAny(context.Context, multihash.Multihash) (acceptance.Acceptance, error)
	// Exists checks if any acceptance exists for a blob (digest).
	Exists(context.Context, multihash.Multihash) (bool, error)
	// Put adds or replaces acceptance data in the store.
	Put(context.Context, acceptance.Acceptance) error
}

// KeyEncoder defines how to encode keys for a specific backend.
type KeyEncoder interface {
	EncodeKey(digest multihash.Multihash, space did.DID) string
	EncodeKeyPrefix(digest multihash.Multihash) string
}

// Store implements AcceptanceStore backed by any ListableStore.
type Store struct {
	store   *genericstore.Store[acceptance.Acceptance]
	encoder KeyEncoder
}

var _ AcceptanceStore = (*Store)(nil)

// New creates an AcceptanceStore with the given backend, prefix, and key encoder.
func New(backend objectstore.ListableStore, prefix string, encoder KeyEncoder) *Store {
	return &Store{
		store:   genericstore.New[acceptance.Acceptance](backend, prefix, acceptance.Codec{}),
		encoder: encoder,
	}
}

func (s *Store) Get(ctx context.Context, digest multihash.Multihash, space did.DID) (acceptance.Acceptance, error) {
	acc, err := s.store.Get(ctx, s.encoder.EncodeKey(digest, space))
	if err != nil {
		return acceptance.Acceptance{}, fmt.Errorf("getting acceptance: %w", err)
	}
	return acc, nil
}

func (s *Store) GetAny(ctx context.Context, digest multihash.Multihash) (acceptance.Acceptance, error) {
	acc, err := s.store.GetAny(ctx, s.encoder.EncodeKeyPrefix(digest))
	if err != nil {
		return acceptance.Acceptance{}, fmt.Errorf("getting any acceptance: %w", err)
	}
	return acc, nil
}

func (s *Store) Exists(ctx context.Context, digest multihash.Multihash) (bool, error) {
	return s.store.ExistsWithPrefix(ctx, s.encoder.EncodeKeyPrefix(digest))
}

func (s *Store) Put(ctx context.Context, acc acceptance.Acceptance) error {
	return s.store.Put(ctx, s.encoder.EncodeKey(acc.Blob.Digest, acc.Space), acc)
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

// NewS3Store creates an AcceptanceStore for S3/MinIO backends.
// Acceptances are stored with keys formatted as "acceptances/{digest}/{space}.cbor".
func NewS3Store(backend *minio.Store) *Store {
	return New(backend, "acceptances/", S3KeyEncoder{})
}

// NewDatastoreStore creates an AcceptanceStore for LevelDB/datastore backends.
// Acceptances are stored with keys formatted as "{digest}/{space}".
func NewDatastoreStore(ds datastore.Datastore) *Store {
	return New(dsadapter.New(ds), "", DatastoreKeyEncoder{})
}
