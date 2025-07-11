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
	"github.com/storacha/piri/pkg/store/receiptstore"
)

var Module = fx.Module("filesystem-store",
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
	),
)

// TODO this likely needs a named fx tag, or it's own unique interface.
func NewAggregatorStore(cfg app.AppConfig, lc fx.Lifecycle) (datastore.Datastore, error) {
	if cfg.Storage.Aggregator.DatastoreDir == "" {
		return nil, fmt.Errorf("no data dir provided for aggregator store")
	}

	ds, err := newDs(cfg.Storage.Aggregator.DatastoreDir)
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

func NewAllocationStore(cfg app.AppConfig, lc fx.Lifecycle) (allocationstore.AllocationStore, error) {
	if cfg.Storage.Allocations.StoreDir == "" {
		return nil, fmt.Errorf("no data dir provided for allocation store")
	}

	ds, err := newDs(cfg.Storage.Allocations.StoreDir)
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

func NewBlobStore(cfg app.AppConfig) (blobstore.Blobstore, error) {
	if cfg.Storage.Blobs.StoreDir == "" {
		return nil, fmt.Errorf("no data dir provided for blob store")
	}
	var tmpDir = cfg.Storage.Blobs.TempDir
	if tmpDir == "" {
		tmpDir = filepath.Join(os.TempDir(), "storage")
	}

	bs, err := blobstore.NewFsBlobstore(cfg.Storage.Blobs.StoreDir, tmpDir)
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

func NewClaimStore(cfg app.AppConfig, lc fx.Lifecycle) (claimstore.ClaimStore, error) {
	if cfg.Storage.Claims.StoreDir == "" {
		return nil, fmt.Errorf("no data dir provided for claim store")
	}

	ds, err := newDs(cfg.Storage.Claims.StoreDir)
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

func NewPublisherStore(cfg app.AppConfig, lc fx.Lifecycle) (store.FullStore, error) {
	if cfg.Storage.Publisher.StoreDir == "" {
		return nil, fmt.Errorf("no data dir provided for publisher store")
	}

	ds, err := newDs(cfg.Storage.Publisher.StoreDir)
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

func NewReceiptStore(cfg app.AppConfig, lc fx.Lifecycle) (receiptstore.ReceiptStore, error) {
	if cfg.Storage.Receipts.StoreDir == "" {
		return nil, fmt.Errorf("no data dir provided for receipt store")
	}

	ds, err := newDs(cfg.Storage.Receipts.StoreDir)
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
