package allocationstore

import (
	"context"

	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/piri/pkg/store/allocationstore/allocation"
)

type ListConfig struct {
	Limit int
}

// ListOption is an option for configuring a call to List.
type ListOption func(c *ListConfig)

// WithLimit configures the maximum number of items that will be returned.
func WithLimit(max int) ListOption {
	return func(c *ListConfig) {
		c.Limit = max
	}
}

// AllocationStore tracks the items that have been, or will soon be stored on
// the storage node.
type AllocationStore interface {
	// Get retrieves an allocation for a blob (digest) in a space (DID). It
	// returns [github.com/storacha/piri/pkg/store.ErrNotFound] if the allocation
	// does not exist.
	Get(context.Context, multihash.Multihash, did.DID) (allocation.Allocation, error)
	// List retrieves allocations by the digest of the data allocated. Note: may
	// return an empty slice.
	List(context.Context, multihash.Multihash, ...ListOption) ([]allocation.Allocation, error)
	// Put adds or replaces allocation data in the store.
	Put(context.Context, allocation.Allocation) error
}
