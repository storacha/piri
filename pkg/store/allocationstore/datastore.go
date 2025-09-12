package allocationstore

import (
	"context"
	"fmt"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	multihash "github.com/multiformats/go-multihash"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/piri/pkg/internal/digestutil"
	"github.com/storacha/piri/pkg/store/allocationstore/allocation"
)

type DsAllocationStore struct {
	data datastore.Datastore
}

func (d *DsAllocationStore) List(ctx context.Context, digest multihash.Multihash) ([]allocation.Allocation, error) {
	pfx := digestutil.Format(digest) + "/"
	return d.listByPrefix(ctx, pfx)
}

func (d *DsAllocationStore) ListBySpace(ctx context.Context, digest multihash.Multihash, space did.DID) ([]allocation.Allocation, error) {
	pfx := digestutil.Format(digest) + "/" + space.String()
	return d.listByPrefix(ctx, pfx)
}

func (d *DsAllocationStore) listByPrefix(ctx context.Context, pfx string) ([]allocation.Allocation, error) {
	results, err := d.data.Query(ctx, query.Query{Prefix: pfx})
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
	k := encodeKey(alloc)
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

func encodeKey(a allocation.Allocation) datastore.Key {
	str := digestutil.Format(a.Blob.Digest)
	return datastore.NewKey(fmt.Sprintf("%s/%s", str, a.Space.String()))
}
