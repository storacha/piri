package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ipfs/go-datastore"
	leveldb "github.com/ipfs/go-ds-leveldb"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-libstoracha/metadata"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/store/claimstore"
	"github.com/storacha/piri/pkg/store/delegationstore"
	"github.com/storacha/piri/pkg/store/keystore"
	"github.com/storacha/piri/pkg/store/receiptstore"
	"github.com/storacha/piri/pkg/store/retrievaljournal"
	"github.com/storacha/piri/pkg/store/stashstore"
)

var Module = fx.Module("filesystem-store",
	fx.Provide(
		ProvideConfigs,
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
		NewRetrievalJournal,
		NewKeyStore,
		NewStashStore,
		NewPDPStore,
	),
)

type Configs struct {
	fx.Out
	Aggregator     app.AggregatorStorageConfig
	Publisher      app.PublisherStorageConfig
	Allocation     app.AllocationStorageConfig
	Blob           app.BlobStorageConfig
	Claim          app.ClaimStorageConfig
	Receipt        app.ReceiptStorageConfig
	EgressTracking app.EgressTrackingStorageConfig
	KeyStore       app.KeyStoreConfig
	Stash          app.StashStoreConfig
	PDP            app.PDPStoreConfig
}

// ProvideConfigs provides the fields of a storage config
func ProvideConfigs(cfg app.StorageConfig) Configs {
	return Configs{
		Aggregator:     cfg.Aggregator,
		Publisher:      cfg.Publisher,
		Allocation:     cfg.Allocations,
		Blob:           cfg.Blobs,
		Claim:          cfg.Claims,
		Receipt:        cfg.Receipts,
		EgressTracking: cfg.EgressTracking,
		KeyStore:       cfg.KeyStore,
		Stash:          cfg.StashStore,
		PDP:            cfg.PDPStore,
	}
}

func NewAggregatorDatastore(cfg app.AggregatorStorageConfig, lc fx.Lifecycle) (datastore.Datastore, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("no data dir provided for aggregator store")
	}

	ds, err := newDs(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("creating aggregator store: %w", err)
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return ds.Close()
		},
	})

	return ds, nil
}

func NewAllocationStore(cfg app.AllocationStorageConfig, lc fx.Lifecycle) (allocationstore.AllocationStore, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("no data dir provided for allocation store")
	}

	ds, err := newDs(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("creating allocation store: %w", err)
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return ds.Close()
		},
	})

	return allocationstore.NewDsAllocationStore(ds)
}

func NewBlobStore(cfg app.BlobStorageConfig) (blobstore.Blobstore, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("no data dir provided for blob store")
	}
	var tmpDir = cfg.TmpDir
	if tmpDir == "" {
		tmpDir = filepath.Join(os.TempDir(), "storage")
	}

	bs, err := blobstore.NewFsBlobstore(cfg.Dir, tmpDir)
	if err != nil {
		return nil, fmt.Errorf("creating blob store: %w", err)
	}
	return bs, nil
	// TODO(forrest): unsure of the purpose of a DS based blobstore, currently not used.
	/*
		ds, err := newDs(cfg.BlobStoreDir)
		if err != nil {
			return nil, fmt.Errorf("creating blob store: %w", err)
		}

		return blobstore.NewDsBlobstore(ds), nil
	*/
}

func NewClaimStore(cfg app.ClaimStorageConfig, lc fx.Lifecycle) (claimstore.ClaimStore, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("no data dir provided for claim store")
	}

	ds, err := newDs(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("creating claim store: %w", err)
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return ds.Close()
		},
	})

	return delegationstore.NewDsDelegationStore(ds)
}

func NewPublisherStore(cfg app.PublisherStorageConfig, lc fx.Lifecycle) (store.FullStore, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("no data dir provided for publisher store")
	}

	ds, err := newDs(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("creating publisher store: %w", err)
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return ds.Close()
		},
	})

	return store.FromDatastore(ds, store.WithMetadataContext(metadata.MetadataContext)), nil
}

func NewReceiptStore(cfg app.ReceiptStorageConfig, lc fx.Lifecycle) (receiptstore.ReceiptStore, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("no data dir provided for receipt store")
	}

	ds, err := newDs(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("creating receipt store: %w", err)
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return ds.Close()
		},
	})

	return receiptstore.NewDsReceiptStore(ds)

}

func NewRetrievalJournal(cfg app.EgressTrackingStorageConfig) (retrievaljournal.Journal, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("no data dir provided for egress batch store")
	}

	return retrievaljournal.NewFSJournal(cfg.Dir, cfg.MaxBatchSize)
}

func NewKeyStore(cfg app.KeyStoreConfig, lc fx.Lifecycle) (keystore.KeyStore, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("no data dir provided for key store")
	}

	ds, err := newDs(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("creating key store: %w", err)
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return ds.Close()
		},
	})
	return keystore.NewKeyStore(ds)
}

func NewStashStore(cfg app.StashStoreConfig) (stashstore.Stash, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("no data dir provided for stash store")
	}
	return stashstore.NewStashStore(cfg.Dir)
}

// TODO whenever we are done with https://github.com/storacha/piri/issues/140
// make this an object store.
// We must do this before production network launch, else migration will be the end of me.
func NewPDPStore(cfg app.PDPStoreConfig, lc fx.Lifecycle) (blobstore.PDPStore, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("no data dir provided for pdp store")
	}
	ds, err := newDs(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("creating pdp store: %w", err)
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return ds.Close()
		},
	})
	return blobstore.NewTODO_DsBlobstore(ds), nil
}

func newDs(path string) (*leveldb.Datastore, error) {
	dirPath, err := mkdirp(path)
	if err != nil {
		return nil, fmt.Errorf("creating leveldb for store at path %s: %w", path, err)
	}
	return leveldb.NewDatastore(dirPath, nil)
}

func mkdirp(dirpath ...string) (string, error) {
	dir := filepath.Join(dirpath...)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return "", fmt.Errorf("creating directory: %s: %w", dir, err)
	}
	return dir, nil
}
