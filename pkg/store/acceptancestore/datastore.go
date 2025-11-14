package acceptancestore

import (
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	multihash "github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/digestutil"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/acceptancestore/acceptance"
)

type DsAcceptanceStore struct {
	data datastore.Datastore
}

func (d *DsAcceptanceStore) Get(ctx context.Context, digest multihash.Multihash, space did.DID) (acceptance.Acceptance, error) {
	value, err := d.data.Get(ctx, datastore.NewKey(encodeKey(digest, space)))
	if err != nil {
		if errors.Is(err, datastore.ErrNotFound) {
			return acceptance.Acceptance{}, store.ErrNotFound
		}
		return acceptance.Acceptance{}, fmt.Errorf("getting from datastore: %w", err)
	}
	return acceptance.Decode(value, dagcbor.Decode)
}

func (d *DsAcceptanceStore) List(ctx context.Context, digest multihash.Multihash, options ...ListOption) ([]acceptance.Acceptance, error) {
	cfg := ListConfig{}
	for _, opt := range options {
		opt(&cfg)
	}

	pfx := encodeKeyPrefix(digest)
	results, err := d.data.Query(ctx, query.Query{Prefix: pfx, Limit: cfg.Limit})
	if err != nil {
		return nil, fmt.Errorf("querying datastore: %w", err)
	}

	var accepts []acceptance.Acceptance
	for entry := range results.Next() {
		if entry.Error != nil {
			return nil, fmt.Errorf("iterating query results: %w", err)
		}
		a, err := acceptance.Decode(entry.Value, dagcbor.Decode)
		if err != nil {
			return nil, fmt.Errorf("decoding data: %w", err)
		}
		accepts = append(accepts, a)
	}
	return accepts, nil
}

func (d *DsAcceptanceStore) Put(ctx context.Context, acc acceptance.Acceptance) error {
	k := datastore.NewKey(encodeKey(acc.Blob.Digest, acc.Space))
	b, err := acceptance.Encode(acc, dagcbor.Encode)
	if err != nil {
		return fmt.Errorf("encoding data: %w", err)
	}

	err = d.data.Put(ctx, k, b)
	if err != nil {
		return fmt.Errorf("writing to datastore: %w", err)
	}

	return nil
}

var _ AcceptanceStore = (*DsAcceptanceStore)(nil)

// NewDsAcceptanceStore creates an [AllocationStore] backed by an IPFS datastore.
func NewDsAcceptanceStore(ds datastore.Datastore) (*DsAcceptanceStore, error) {
	return &DsAcceptanceStore{ds}, nil
}

func encodeKey(digest multihash.Multihash, space did.DID) string {
	return fmt.Sprintf("%s/%s", digestutil.Format(digest), space.String())
}

func encodeKeyPrefix(digest multihash.Multihash) string {
	return fmt.Sprintf("%s/", digestutil.Format(digest))
}
