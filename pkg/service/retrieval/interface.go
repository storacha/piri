package retrieval

import (
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
)

type Service interface {
	// ID is the service identity, used to sign UCAN invocations and receipts.
	ID() principal.Signer
	// Allocations is the store for received blob allocations.
	Allocations() allocationstore.AllocationStore
	// Blobs is the storage interface for retrieving blobs. It MUST be keyed by
	// hash that a client will request. i.e. not a piece hash.
	Blobs() blobstore.BlobGetter
}
