package s3

import (
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/store/acceptancestore"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/store/claimstore"
	"github.com/storacha/piri/pkg/store/consolidationstore"
	"github.com/storacha/piri/pkg/store/delegationstore"
	minio_store "github.com/storacha/piri/pkg/store/objectstore/minio"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

// Module provides stores backed by S3-compatible storage.
// Use this module alongside filesystem.LocalOnlyModule which provides
// stores that must remain on the local filesystem (AggregatorDatastore,
// PublisherStore, RetrievalJournal, KeyStore).
var Module = fx.Module("s3-store",
	fx.Provide(
		ProvideConfigs,
		NewStores,
		NewAllocationStore,
		NewAcceptanceStore,
		NewClaimStore,
		NewReceiptStore,
		fx.Annotate(
			NewPDPStore,
			fx.As(fx.Self()),
			fx.As(new(blobstore.BlobGetter)),
		),
		NewConsolidationStore,
	),
)

// Stores holds all S3/MinIO store instances for different store types.
// Each store uses a separate bucket named with the configured prefix.
type Stores struct {
	Allocations   *minio_store.Store
	Acceptances   *minio_store.Store
	Claims        *minio_store.Store
	Receipts      *minio_store.Store
	PDP           *minio_store.Store
	Consolidation *minio_store.Store
}

// NewStores creates S3/MinIO stores for each store type.
// Each store gets its own bucket named with the configured prefix:
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

// Configs provides storage configs needed by S3-backed stores.
type Configs struct {
	fx.Out
	Allocation    app.AllocationStorageConfig
	Blob          app.BlobStorageConfig
	Claim         app.ClaimStorageConfig
	Receipt       app.ReceiptStorageConfig
	Stash         app.StashStoreConfig
	PDP           app.PDPStoreConfig
	Acceptance    app.AcceptanceStorageConfig
	Consolidation app.ConsolidationStorageConfig
}

// ProvideConfigs extracts configs for S3-backed stores.
func ProvideConfigs(cfg app.StorageConfig) Configs {
	return Configs{
		Allocation:    cfg.Allocations,
		Blob:          cfg.Blobs,
		Claim:         cfg.Claims,
		Receipt:       cfg.Receipts,
		Stash:         cfg.StashStore,
		PDP:           cfg.PDPStore,
		Acceptance:    cfg.Acceptance,
		Consolidation: cfg.Consolidation,
	}
}

func NewAllocationStore(stores *Stores) allocationstore.AllocationStore {
	return allocationstore.NewS3Store(stores.Allocations)
}

func NewAcceptanceStore(stores *Stores) acceptancestore.AcceptanceStore {
	return acceptancestore.NewS3Store(stores.Acceptances)
}

func NewClaimStore(stores *Stores) claimstore.ClaimStore {
	return delegationstore.NewS3Store(stores.Claims)
}

func NewReceiptStore(stores *Stores) receiptstore.ReceiptStore {
	return receiptstore.NewS3Store(stores.Receipts)
}

// NewPDPStore provides the blob store. It also satisfies blobstore.BlobGetter.
func NewPDPStore(stores *Stores) blobstore.Blobstore {
	return blobstore.NewS3Store(stores.PDP)
}

func NewConsolidationStore(stores *Stores) consolidationstore.Store {
	return consolidationstore.NewS3Store(stores.Consolidation)
}
