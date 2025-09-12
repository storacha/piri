package allocationstore

import (
	"context"

	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/piri/pkg/store/allocationstore/allocation"
)

// AllocationStore tracks the items that have been, or will soon be stored on
// the storage node.
type AllocationStore interface {
	// List retrieves allocations by the digest of the data allocated.
	List(context.Context, multihash.Multihash) ([]allocation.Allocation, error)
	// List retrieves allocations by the digest of the data allocated and the
	// space they are allocated in.
	ListBySpace(context.Context, multihash.Multihash, did.DID) ([]allocation.Allocation, error)
	// Put adds or replaces allocation data in the store.
	Put(context.Context, allocation.Allocation) error
}
