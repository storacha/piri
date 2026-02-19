package s3

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ipfs/go-datastore"
	leveldb "github.com/ipfs/go-ds-leveldb"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-libstoracha/metadata"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/store/acceptancestore"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/store/claimstore"
	"github.com/storacha/piri/pkg/store/consolidationstore"
	"github.com/storacha/piri/pkg/store/delegationstore"
	minio_store "github.com/storacha/piri/pkg/store/objectstore/minio"
	"github.com/storacha/piri/pkg/store/local/retrievaljournal"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

// Module provides all stores backed by S3-compatible storage.
// Note: KeyStore is NOT provided - use filesystem.KeyStoreModule alongside this.
var Module = fx.Module("s3-store",
	fx.Provide(
		ProvideConfigs,
		NewStores,
		fx.Annotate(
			NewAggregatorDatastore,
			fx.ResultTags(`name:"aggregator_datastore"`),
		),
		fx.Annotate(
			NewPublisherStore,
			fx.As(fx.Self()),
			fx.As(new(store.PublisherStore)),
			fx.As(new(store.EncodeableStore)),
		),
		NewAllocationStore,
		NewAcceptanceStore,
		fx.Annotate(
			NewBlobStore,
			fx.As(fx.Self()),
			fx.As(new(blobstore.BlobGetter)),
		),
		NewClaimStore,
		NewReceiptStore,
		NewRetrievalJournal,
		fx.Annotate(
			NewPDPStore,
			// tagged as pdp_store since PDPStore is now an alias to Blobstore
			fx.ResultTags(`name:"pdp_store"`),
		),
		NewConsolidationStore,
		// Note: KeyStore is NOT provided here - it must always be on disk
	),
)

// Stores holds all S3/MinIO store instances for different store types.
// Each store uses a separate bucket named with the configured prefix.
type Stores struct {
	Blobs         *minio_store.Store
	Allocations   *minio_store.Store
	Acceptances   *minio_store.Store
	Claims        *minio_store.Store
	Receipts      *minio_store.Store
	PDP           *minio_store.Store
	Consolidation *minio_store.Store
}

// NewStores creates S3/MinIO stores for each store type.
// Each store gets its own bucket named with the configured prefix:
// - {prefix}blobs
// - {prefix}allocations
// - {prefix}acceptances
// - {prefix}claims
// - {prefix}receipts
// - {prefix}pdp
// - {prefix}consolidation
func NewStores(cfg app.StorageConfig) (*Stores, error) {
	if cfg.S3 == nil || cfg.S3.Endpoint == "" || cfg.S3.BucketPrefix == "" {
		return nil, fmt.Errorf("S3 configuration required: endpoint and bucket_prefix must be set")
	}

	options := minio.Options{Secure: !cfg.S3.Insecure}
	if cfg.S3.Credentials.AccessKeyID != "" && cfg.S3.Credentials.SecretAccessKey != "" {
		options.Creds = credentials.NewStaticV4(
			cfg.S3.Credentials.AccessKeyID,
			cfg.S3.Credentials.SecretAccessKey,
			"",
		)
	}

	prefix := cfg.S3.BucketPrefix
	endpoint := cfg.S3.Endpoint
	stores := &Stores{}
	var err error

	// Create a store for each bucket
	if stores.Blobs, err = minio_store.New(endpoint, prefix+"blobs", options); err != nil {
		return nil, fmt.Errorf("creating blobs s3 store: %w", err)
	}
	if stores.Allocations, err = minio_store.New(endpoint, prefix+"allocations", options); err != nil {
		return nil, fmt.Errorf("creating allocations s3 store: %w", err)
	}
	if stores.Acceptances, err = minio_store.New(endpoint, prefix+"acceptances", options); err != nil {
		return nil, fmt.Errorf("creating acceptances s3 store: %w", err)
	}
	if stores.Claims, err = minio_store.New(endpoint, prefix+"claims", options); err != nil {
		return nil, fmt.Errorf("creating claims s3 store: %w", err)
	}
	if stores.Receipts, err = minio_store.New(endpoint, prefix+"receipts", options); err != nil {
		return nil, fmt.Errorf("creating receipts s3 store: %w", err)
	}
	if stores.PDP, err = minio_store.New(endpoint, prefix+"pdp", options); err != nil {
		return nil, fmt.Errorf("creating pdp s3 store: %w", err)
	}
	if stores.Consolidation, err = minio_store.New(endpoint, prefix+"consolidation", options); err != nil {
		return nil, fmt.Errorf("creating consolidation s3 store: %w", err)
	}

	return stores, nil
}

type Configs struct {
	fx.Out
	Aggregator    app.AggregatorStorageConfig
	Publisher     app.PublisherStorageConfig
	Allocation    app.AllocationStorageConfig
	Blob          app.BlobStorageConfig
	Claim         app.ClaimStorageConfig
	Receipt       app.ReceiptStorageConfig
	EgressTracker app.EgressTrackerStorageConfig
	KeyStore      app.KeyStoreConfig
	Stash         app.StashStoreConfig
	PDP           app.PDPStoreConfig
	Acceptance    app.AcceptanceStorageConfig
	Consolidation app.ConsolidationStorageConfig
}

// ProvideConfigs provides the fields of a storage config
func ProvideConfigs(cfg app.StorageConfig) Configs {
	return Configs{
		Aggregator:    cfg.Aggregator,
		Publisher:     cfg.Publisher,
		Allocation:    cfg.Allocations,
		Blob:          cfg.Blobs,
		Claim:         cfg.Claims,
		Receipt:       cfg.Receipts,
		EgressTracker: cfg.EgressTracker,
		KeyStore:      cfg.KeyStore,
		Stash:         cfg.StashStore,
		PDP:           cfg.PDPStore,
		Acceptance:    cfg.Acceptance,
		Consolidation: cfg.Consolidation,
	}
}

// NewAggregatorDatastore uses leveldb (no S3 implementation available)
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

// NewPublisherStore uses leveldb (no S3 implementation available)
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

func NewAllocationStore(stores *Stores) allocationstore.AllocationStore {
	return allocationstore.NewS3Store(stores.Allocations)
}

func NewAcceptanceStore(stores *Stores) acceptancestore.AcceptanceStore {
	return acceptancestore.NewS3Store(stores.Acceptances)
}

func NewBlobStore(stores *Stores) blobstore.Blobstore {
	return blobstore.NewS3Store(stores.Blobs)
}

func NewClaimStore(stores *Stores) claimstore.ClaimStore {
	return delegationstore.NewS3Store(stores.Claims)
}

func NewReceiptStore(stores *Stores) receiptstore.ReceiptStore {
	return receiptstore.NewS3Store(stores.Receipts)
}

func NewRetrievalJournal(storeCfg app.EgressTrackerStorageConfig, svcCfg app.UCANServiceConfig, lc fx.Lifecycle) (retrievaljournal.Journal, error) {
	if storeCfg.Dir == "" {
		return nil, fmt.Errorf("no data dir provided for retrieval journal")
	}

	rj, err := retrievaljournal.NewFSJournal(storeCfg.Dir, svcCfg.Services.EgressTracker.MaxBatchSizeBytes)
	if err != nil {
		return nil, fmt.Errorf("creating retrieval journal: %w", err)
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return rj.Close()
		},
	})

	return rj, nil
}

// NewPDPStore is annotated with name:"pdp_store" in the module.
func NewPDPStore(stores *Stores) blobstore.Blobstore {
	return blobstore.NewS3Store(stores.PDP)
}

func NewConsolidationStore(stores *Stores) consolidationstore.Store {
	return consolidationstore.NewS3Store(stores.Consolidation)
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
