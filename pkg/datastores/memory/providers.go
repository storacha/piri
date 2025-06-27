package memory

import (
	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-libstoracha/metadata"

	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/store/claimstore"
	"github.com/storacha/piri/pkg/store/delegationstore"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

var log = logging.Logger("memory-datastores")

// ProvideBlobstore creates a blobstore based on the provided configuration
func ProvideBlobstore() (blobstore.Blobstore, error) {
	return blobstore.NewMapBlobstore(), nil
}

func ProvideAllocationStore() (allocationstore.AllocationStore, error) {
	ds := datastore.NewMapDatastore()
	return allocationstore.NewDsAllocationStore(ds)
}

func ProvideClaimsStore() (claimstore.ClaimStore, error) {
	ds := datastore.NewMapDatastore()
	return delegationstore.NewDsDelegationStore(ds)
}

func ProvidePublisherStore() (store.PublisherStore, error) {
	ds := datastore.NewMapDatastore()
	return store.FromDatastore(ds, store.WithMetadataContext(metadata.MetadataContext)), nil
}

func ProvideReceiptStore() (receiptstore.ReceiptStore, error) {
	ds := datastore.NewMapDatastore()
	return receiptstore.NewDsReceiptStore(ds)
}
