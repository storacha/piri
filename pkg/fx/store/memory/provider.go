package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-libstoracha/metadata"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/store/claimstore"
	"github.com/storacha/piri/pkg/store/delegationstore"
	"github.com/storacha/piri/pkg/store/keystore"
	"github.com/storacha/piri/pkg/store/receiptstore"
	"github.com/storacha/piri/pkg/store/stashstore"
)

var Module = fx.Module("memory-store",
	fx.Provide(
		fx.Annotate(
			NewAggregatorDatastore,
			// tagged as aggregator_datastore since this returns a datastore.Datastore which is too generic to
			// safely provide as is.
			fx.ResultTags(`name:"aggregator_datastore"`),
		),
		fx.Annotate(
			NewPublisherStore,
			// provide as FullStore (self)
			fx.As(fx.Self()),
			// provide sub-interfaces of Full Store
			fx.As(new(store.PublisherStore)),
			fx.As(new(store.EncodeableStore)),
		),
		NewAllocationStore,
		fx.Annotate(
			NewBlobStore,
			// provide as Blobstore (self)
			fx.As(fx.Self()),
			// provide sub-interfaces of Blobstore
			fx.As(new(blobstore.BlobGetter)),
		),
		NewClaimStore,
		NewReceiptStore,
		NewKeyStore,
		NewStashStore,
		NewPDPStore,
	),
)

func NewAggregatorDatastore() datastore.Datastore {
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
func NewStashStore(lc fx.Lifecycle) (stashstore.Stash, error) {
	tmpDir := filepath.Join(os.TempDir(), "piri-stash-tmp")
	out, err := stashstore.NewStashStore(tmpDir)
	if err != nil {
		return nil, fmt.Errorf("creating stash store")
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return os.RemoveAll(tmpDir)
		},
	})
	return out, nil
}

func NewPDPStore() (blobstore.PDPStore, error) {
	ds := sync.MutexWrap(datastore.NewMapDatastore())
	return blobstore.NewTODO_DsBlobstore(ds), nil
}
