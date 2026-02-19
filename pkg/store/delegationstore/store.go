package delegationstore

import (
	"context"
	"fmt"
	"io"

	"github.com/ipfs/go-datastore"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/ucan"

	"github.com/storacha/piri/pkg/store/genericstore"
	"github.com/storacha/piri/pkg/store/objectstore"
	"github.com/storacha/piri/pkg/store/objectstore/dsadapter"
	"github.com/storacha/piri/pkg/store/objectstore/minio"
)

// DelegationStore stores UCAN delegations.
type DelegationStore interface {
	// Get retrieves a delegation by its root CID.
	Get(context.Context, ucan.Link) (delegation.Delegation, error)
	// Put adds or replaces a delegation in the store.
	Put(context.Context, delegation.Delegation) error
}

// KeyEncoder defines how to encode keys for a specific backend.
type KeyEncoder interface {
	EncodeKey(link ucan.Link) string
}

// Store implements DelegationStore backed by genericstore.
type Store struct {
	store   *genericstore.Store[delegation.Delegation]
	encoder KeyEncoder
}

var _ DelegationStore = (*Store)(nil)

// New creates a DelegationStore with the given backend, prefix, and key encoder.
func New(backend objectstore.ListableStore, prefix string, encoder KeyEncoder) *Store {
	return &Store{
		store:   genericstore.New[delegation.Delegation](backend, prefix, Codec{}),
		encoder: encoder,
	}
}

func (s *Store) Get(ctx context.Context, link ucan.Link) (delegation.Delegation, error) {
	dlg, err := s.store.Get(ctx, s.encoder.EncodeKey(link))
	if err != nil {
		return nil, fmt.Errorf("getting delegation: %w", err)
	}
	return dlg, nil
}

func (s *Store) Put(ctx context.Context, dlg delegation.Delegation) error {
	return s.store.Put(ctx, s.encoder.EncodeKey(dlg.Link()), dlg)
}

// Codec implements genericstore.Codec for delegation.Delegation.
type Codec struct{}

func (Codec) Encode(dlg delegation.Delegation) ([]byte, error) {
	return io.ReadAll(dlg.Archive())
}

func (Codec) Decode(data []byte) (delegation.Delegation, error) {
	return delegation.Extract(data)
}

// S3KeyEncoder encodes keys for S3/MinIO backends.
type S3KeyEncoder struct{}

func (S3KeyEncoder) EncodeKey(link ucan.Link) string {
	return link.String()
}

// DatastoreKeyEncoder encodes keys for LevelDB/datastore backends.
type DatastoreKeyEncoder struct{}

func (DatastoreKeyEncoder) EncodeKey(link ucan.Link) string {
	return link.String()
}

// NewS3Store creates a DelegationStore for S3/MinIO backends.
// Delegations are stored with keys formatted as "delegations/{cid}".
func NewS3Store(backend *minio.Store) *Store {
	return New(backend, "delegations/", S3KeyEncoder{})
}

// NewDatastoreStore creates a DelegationStore for LevelDB/datastore backends.
// Delegations are stored with keys formatted as "{cid}".
func NewDatastoreStore(ds datastore.Datastore) *Store {
	return New(dsadapter.New(ds), "", DatastoreKeyEncoder{})
}
