package acceptancestore

import (
	"context"

	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/piri/pkg/store/acceptancestore/acceptance"
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

// AcceptanceStore tracks the items that have been stored on the storage node.
type AcceptanceStore interface {
	// Get retrieves an acceptance for a blob (digest) in a space (DID). It
	// returns [github.com/storacha/piri/pkg/store.ErrNotFound] if the allocation
	// does not exist.
	Get(context.Context, multihash.Multihash, did.DID) (acceptance.Acceptance, error)
	// List retrieves allocations by the digest of the data allocated. Note: may
	// return an empty slice.
	List(context.Context, multihash.Multihash, ...ListOption) ([]acceptance.Acceptance, error)
	// Put adds or replaces allocation data in the store.
	Put(context.Context, acceptance.Acceptance) error
}
