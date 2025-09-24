package allocationstore

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
	"github.com/storacha/piri/pkg/store/allocationstore/allocation"
)

type DsAllocationStore struct {
	data datastore.Datastore
}

func (d *DsAllocationStore) Get(ctx context.Context, digest multihash.Multihash, space did.DID) (allocation.Allocation, error) {
	value, err := d.data.Get(ctx, datastore.NewKey(encodeKey(digest, space)))
	if err != nil {
		if errors.Is(err, datastore.ErrNotFound) {
			return allocation.Allocation{}, store.ErrNotFound
		}
		return allocation.Allocation{}, fmt.Errorf("getting from datastore: %w", err)
	}
	return allocation.Decode(value, dagcbor.Decode)
}

func (d *DsAllocationStore) List(ctx context.Context, digest multihash.Multihash, options ...ListOption) ([]allocation.Allocation, error) {
	cfg := ListConfig{}
	for _, opt := range options {
		opt(&cfg)
	}

	pfx := encodeKeyPrefix(digest)
	results, err := d.data.Query(ctx, query.Query{Prefix: pfx, Limit: cfg.Limit})
	if err != nil {
		return nil, fmt.Errorf("querying datastore: %w", err)
	}

	var allocs []allocation.Allocation
	for entry := range results.Next() {
		if entry.Error != nil {
			return nil, fmt.Errorf("iterating query results: %w", err)
		}
		a, err := allocation.Decode(entry.Value, dagcbor.Decode)
		if err != nil {
			return nil, fmt.Errorf("decoding data: %w", err)
		}
		allocs = append(allocs, a)
	}
	return allocs, nil
}

func (d *DsAllocationStore) Put(ctx context.Context, alloc allocation.Allocation) error {
	k := datastore.NewKey(encodeKey(alloc.Blob.Digest, alloc.Space))
	b, err := allocation.Encode(alloc, dagcbor.Encode)
	if err != nil {
		return fmt.Errorf("encoding data: %w", err)
	}

	err = d.data.Put(ctx, k, b)
	if err != nil {
		return fmt.Errorf("writing to datastore: %w", err)
	}

	return nil
}

var _ AllocationStore = (*DsAllocationStore)(nil)

// NewDsAllocationStore creates an [AllocationStore] backed by an IPFS datastore.
func NewDsAllocationStore(ds datastore.Datastore) (*DsAllocationStore, error) {
	return &DsAllocationStore{ds}, nil
}

func encodeKey(digest multihash.Multihash, space did.DID) string {
	return fmt.Sprintf("%s/%s", digestutil.Format(digest), space.String())
}

func encodeKeyPrefix(digest multihash.Multihash) string {
	return fmt.Sprintf("%s/", digestutil.Format(digest))
}
