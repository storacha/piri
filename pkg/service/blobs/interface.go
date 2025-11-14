package blobs

import (
	"github.com/storacha/piri/pkg/access"
	"github.com/storacha/piri/pkg/presigner"
	"github.com/storacha/piri/pkg/store/acceptancestore"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
)

type Blobs interface {
	// Blobs is the storage interface for blobs.
	Store() blobstore.Blobstore
	// Allocations is a store for received blob allocations.
	Allocations() allocationstore.AllocationStore
	// Acceptances is a store that records accepted blobs.
	Acceptances() acceptancestore.AcceptanceStore
	// Presigner provides an interface to allow signed request access to upload blobs.
	Presigner() presigner.RequestPresigner
	// Access provides an interface to allowing public access to download blobs.
	Access() access.Access
}
