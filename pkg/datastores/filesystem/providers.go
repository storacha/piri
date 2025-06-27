package filesystem

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/ipfs/go-datastore"
	leveldb "github.com/ipfs/go-ds-leveldb"
	logging "github.com/ipfs/go-log/v2"
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

var log = logging.Logger("filesystem-datastores")

// ProvideBlobstore creates a blobstore based on the provided configuration
func ProvideBlobstore(cfg app.Config, lc fx.Lifecycle) (blobstore.Blobstore, error) {
	if cfg.DataDir == "" {
		log.Warn("no data directory provided, using memory blobstore store")
		return blobstore.NewMapBlobstore(), nil
	}
	// NB: NewFSBlobstore will create a temp dir if cfg.TempDir == ""
	return blobstore.NewFsBlobstore(cfg.DataDir, cfg.TempDir)
}

func ProvideAllocationStore(cfg app.Config, lc fx.Lifecycle) (allocationstore.AllocationStore, error) {
	var ds datastore.Datastore
	if cfg.DataDir == "" {
		log.Warn("no data directory provided, using memory allocation store")
		ds = datastore.NewMapDatastore()
	} else {
		allocsDir, err := app.Mkdirp(filepath.Join(cfg.DataDir, "allocation"))
		if err != nil {
			return nil, fmt.Errorf("could not create allocation directory: %w", err)
		}

		ds, err = leveldb.NewDatastore(allocsDir, nil)
		if err != nil {
			return nil, err
		}

		// Only register cleanup for persistent stores
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				log.Info("closing allocation datastore")
				return ds.Close()
			},
		})
	}

	return allocationstore.NewDsAllocationStore(ds)
}

func ProvideClaimsStore(cfg app.Config, lc fx.Lifecycle) (claimstore.ClaimStore, error) {
	var ds datastore.Datastore
	if cfg.DataDir == "" {
		log.Warn("no data directory provided, using memory claim store")
		ds = datastore.NewMapDatastore()
	} else {
		claimsDir, err := app.Mkdirp(filepath.Join(cfg.DataDir, "claims"))
		if err != nil {
			return nil, fmt.Errorf("could not create claims directory: %w", err)
		}

		ds, err = leveldb.NewDatastore(claimsDir, nil)
		if err != nil {
			return nil, err
		}

		// Only register cleanup for persistent stores
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				log.Info("closing claims datastore")
				return ds.Close()
			},
		})
	}
	return delegationstore.NewDsDelegationStore(ds)
}

func ProvidePublisherStore(cfg app.Config, lc fx.Lifecycle) (store.PublisherStore, error) {
	var ds datastore.Datastore
	if cfg.DataDir == "" {
		log.Warn("no data directory provided, using memory publisher store")
		ds = datastore.NewMapDatastore()
	} else {
		publisherDir, err := app.Mkdirp(filepath.Join(cfg.DataDir, "publisher"))
		if err != nil {
			return nil, fmt.Errorf("could not create publisher directory: %w", err)
		}
		ds, err = leveldb.NewDatastore(publisherDir, nil)
		if err != nil {
			return nil, err
		}
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				log.Info("closing publisher datastore")
				return ds.Close()
			},
		})
	}
	return store.FromDatastore(ds, store.WithMetadataContext(metadata.MetadataContext)), nil
}

func ProvideReceiptStore(cfg app.Config, lc fx.Lifecycle) (receiptstore.ReceiptStore, error) {
	var ds datastore.Datastore
	if cfg.DataDir == "" {
		log.Warn("no data directory provided, using memory receipt store")
		ds = datastore.NewMapDatastore()
	} else {
		receiptsDir, err := app.Mkdirp(filepath.Join(cfg.DataDir, "receipts"))
		if err != nil {
			return nil, fmt.Errorf("could not create receipts directory: %w", err)
		}

		ds, err = leveldb.NewDatastore(receiptsDir, nil)
		if err != nil {
			return nil, err
		}

		// Only register cleanup for persistent stores
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				log.Info("closing receipt datastore")
				return ds.Close()
			},
		})
	}

	return receiptstore.NewDsReceiptStore(ds)
}
