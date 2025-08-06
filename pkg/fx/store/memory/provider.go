package memory

import (
	"os"
	"path/filepath"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-libstoracha/metadata"
	"go.uber.org/fx"

	stash "github.com/storacha/piri/pkg/pdp/store"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/store/claimstore"
	"github.com/storacha/piri/pkg/store/delegationstore"
	"github.com/storacha/piri/pkg/store/keystore"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

var Module = fx.Module("memory-store",
	fx.Provide(
		NewAggregatorStore,
		NewAllocationStore,
		NewBlobStore,
		NewClaimStore,
		NewPublisherStore,
		// Also provide the interface
		fx.Annotate(
			func(s store.FullStore) store.PublisherStore {
				return s
			},
		),
		NewReceiptStore,
		NewKeyStore,
		NewStashStore,
		NewPDPStore,
	),
)

func NewAggregatorStore() datastore.Datastore {
	return sync.MutexWrap(datastore.NewMapDatastore())
}

func NewAllocationStore() (allocationstore.AllocationStore, error) {
	ds := sync.MutexWrap(datastore.NewMapDatastore())
	return allocationstore.NewDsAllocationStore(ds)
}

func NewBlobStore() blobstore.Blobstore {
	ds := sync.MutexWrap(datastore.NewMapDatastore())
	return blobstore.NewDsBlobstore(ds)
}

func NewClaimStore() (claimstore.ClaimStore, error) {
	ds := sync.MutexWrap(datastore.NewMapDatastore())
	return delegationstore.NewDsDelegationStore(ds)
}

func NewPublisherStore() store.FullStore {
	ds := sync.MutexWrap(datastore.NewMapDatastore())
	return store.FromDatastore(ds, store.WithMetadataContext(metadata.MetadataContext))
}

func NewReceiptStore() (receiptstore.ReceiptStore, error) {
	ds := sync.MutexWrap(datastore.NewMapDatastore())
	return receiptstore.NewDsReceiptStore(ds)
}

func NewKeyStore() (keystore.KeyStore, error) {
	ds := sync.MutexWrap(datastore.NewMapDatastore())
	return keystore.NewKeyStore(ds)
}

// TODO need an in-memory impl of the stash store...
func NewStashStore() (stash.Stash, error) {
	return stash.NewStashStore(filepath.Join(os.TempDir(), "piri-stash"))
}

func NewPDPStore() (blobstore.PDPStore, error) {
	ds := sync.MutexWrap(datastore.NewMapDatastore())
	return blobstore.NewTODO_DsBlobstore(ds), nil
}
