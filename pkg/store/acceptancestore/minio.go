package acceptancestore

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
	"github.com/storacha/piri/pkg/store/acceptancestore/acceptance"
	"github.com/storacha/piri/pkg/store/objectstore"
	"github.com/storacha/piri/pkg/store/objectstore/minio"
)

const acceptancesPrefix = "acceptances/"

// MinioAcceptanceStore implements AcceptanceStore using a MinIO S3-compatible backend.
type MinioAcceptanceStore struct {
	store  *minio.Store
	prefix string
}

var _ AcceptanceStore = (*MinioAcceptanceStore)(nil)

// NewMinioAcceptanceStore creates an AcceptanceStore backed by a MinIO S3-compatible store.
// Acceptances are stored with keys formatted as "acceptances/{digest}/{space}.cbor".
func NewMinioAcceptanceStore(s *minio.Store) *MinioAcceptanceStore {
	return &MinioAcceptanceStore{
		store:  s,
		prefix: acceptancesPrefix,
	}
}

func (m *MinioAcceptanceStore) Get(ctx context.Context, digest multihash.Multihash, space did.DID) (acceptance.Acceptance, error) {
	key := m.encodeKey(digest, space)
	obj, err := m.store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, objectstore.ErrNotExist) {
			return acceptance.Acceptance{}, store.ErrNotFound
		}
		return acceptance.Acceptance{}, fmt.Errorf("getting acceptance from minio: %w", err)
	}
	defer obj.Body().Close()

	data, err := io.ReadAll(obj.Body())
	if err != nil {
		return acceptance.Acceptance{}, fmt.Errorf("reading acceptance data: %w", err)
	}

	return acceptance.Decode(data, dagcbor.Decode)
}

func (m *MinioAcceptanceStore) List(ctx context.Context, digest multihash.Multihash, options ...ListOption) ([]acceptance.Acceptance, error) {
	cfg := ListConfig{}
	for _, opt := range options {
		opt(&cfg)
	}

	prefix := m.encodeKeyPrefix(digest)
	var accs []acceptance.Acceptance
	count := 0

	for key, err := range m.store.ListPrefix(ctx, prefix) {
		if err != nil {
			return nil, fmt.Errorf("listing acceptances: %w", err)
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
			return nil, fmt.Errorf("getting acceptance %s: %w", key, err)
		}

		data, err := io.ReadAll(obj.Body())
		obj.Body().Close()
		if err != nil {
			return nil, fmt.Errorf("reading acceptance data: %w", err)
		}

		acc, err := acceptance.Decode(data, dagcbor.Decode)
		if err != nil {
			return nil, fmt.Errorf("decoding acceptance: %w", err)
		}

		accs = append(accs, acc)
		count++
	}

	return accs, nil
}

func (m *MinioAcceptanceStore) Put(ctx context.Context, acc acceptance.Acceptance) error {
	key := m.encodeKey(acc.Blob.Digest, acc.Space)

	data, err := acceptance.Encode(acc, dagcbor.Encode)
	if err != nil {
		return fmt.Errorf("encoding acceptance: %w", err)
	}

	err = m.store.Put(ctx, key, uint64(len(data)), bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("writing acceptance to minio: %w", err)
	}

	return nil
}

// encodeKey creates the S3 object key for an acceptance.
// Format: {prefix}{digest}/{space}.cbor
func (m *MinioAcceptanceStore) encodeKey(digest multihash.Multihash, space did.DID) string {
	return fmt.Sprintf("%s%s/%s.cbor", m.prefix, digestutil.Format(digest), space.String())
}

// encodeKeyPrefix creates the S3 prefix for listing acceptances by digest.
// Format: {prefix}{digest}/
func (m *MinioAcceptanceStore) encodeKeyPrefix(digest multihash.Multihash) string {
	return fmt.Sprintf("%s%s/", m.prefix, digestutil.Format(digest))
}
