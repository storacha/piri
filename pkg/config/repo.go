package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/storacha/piri/pkg/config/app"
)

type Credentials struct {
	AccessKeyID     string `mapstructure:"access_key_id" validate:"required" toml:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key" validate:"required" toml:"secret_access_key"`
}

type MinioConfig struct {
	Endpoint    string      `mapstructure:"endpoint" validate:"required" toml:"endpoint"`
	Bucket      string      `mapstructure:"bucket" validate:"required" toml:"bucket"`
	Credentials Credentials `mapstructure:"credentials" toml:"credentials,omitempty"`
	Insecure    bool        `mapstructure:"insecure" toml:"insecure,omitempty"`
}

// BlobStorageConfig is special configuration allowing blobs to be stored
// outside the main repo or on a remote device.
type BlobStorageConfig struct {
	Minio MinioConfig `mapstructure:"minio" toml:"minio,omitempty"`
}

type RepoConfig struct {
	DataDir     string             `mapstructure:"data_dir" validate:"required" flag:"data-dir" toml:"data_dir"`
	TempDir     string             `mapstructure:"temp_dir" validate:"required" flag:"temp-dir" toml:"temp_dir"`
	BlobStorage *BlobStorageConfig `mapstructure:"blob_storage" validate:"omitempty" toml:"blob_storage,omitempty"`
}

func (r RepoConfig) Validate() error {
	return validateConfig(r)
}

func (r RepoConfig) ToAppConfig() (app.StorageConfig, error) {
	if r.DataDir == "" {
		// Return empty config for memory stores
		return app.StorageConfig{}, nil
	}

	// Blob storage is optional; only populate Minio settings when provided.
	var pdpMinio app.MinioConfig
	if r.BlobStorage != nil {
		pdpMinio = app.MinioConfig{
			Endpoint:    r.BlobStorage.Minio.Endpoint,
			Bucket:      r.BlobStorage.Minio.Bucket,
			Credentials: app.Credentials(r.BlobStorage.Minio.Credentials),
			Insecure:    r.BlobStorage.Minio.Insecure,
		}
	}

	// Ensure directories exist
	if err := os.MkdirAll(r.DataDir, 0755); err != nil {
		return app.StorageConfig{}, err
	}
	if err := os.MkdirAll(r.TempDir, 0755); err != nil {
		return app.StorageConfig{}, err
	}

	out := app.StorageConfig{
		DataDir: r.DataDir,
		TempDir: r.TempDir,
		Aggregator: app.AggregatorStorageConfig{
			Dir:    filepath.Join(r.DataDir, "aggregator", "datastore"),
			DBPath: filepath.Join(r.DataDir, "aggregator", "jobqueue", "jobqueue.db"),
		},
		Blobs: app.BlobStorageConfig{
			Dir:    filepath.Join(r.DataDir, "blobs"),
			TmpDir: filepath.Join(r.TempDir, "storage"),
		},
		Claims: app.ClaimStorageConfig{
			Dir: filepath.Join(r.DataDir, "claim"),
		},
		Publisher: app.PublisherStorageConfig{
			Dir: filepath.Join(r.DataDir, "publisher"),
		},
		Receipts: app.ReceiptStorageConfig{
			Dir: filepath.Join(r.DataDir, "receipt"),
		},
		EgressTracker: app.EgressTrackerStorageConfig{
			Dir:    filepath.Join(r.DataDir, "egress_tracker", "journal"),
			DBPath: filepath.Join(r.DataDir, "egress_tracker", "jobqueue", "jobqueue.db"),
		},
		Allocations: app.AllocationStorageConfig{
			Dir: filepath.Join(r.DataDir, "allocation"),
		},
		Acceptance: app.AcceptanceStorageConfig{
			Dir: filepath.Join(r.DataDir, "acceptance"),
		},
		Replicator: app.ReplicatorStorageConfig{
			DBPath: filepath.Join(r.DataDir, "replicator", "replicator.db"),
		},
		KeyStore: app.KeyStoreConfig{
			Dir: filepath.Join(r.DataDir, "wallet"),
		},
		StashStore: app.StashStoreConfig{
			Dir: filepath.Join(r.DataDir, "pdp"),
		},
		SchedulerStorage: app.SchedulerConfig{
			DBPath: filepath.Join(r.DataDir, "pdp", "state", "state.db"),
		},
		PDPStore: app.PDPStoreConfig{
			Dir:   filepath.Join(r.DataDir, "pdp", "datastore"),
			Minio: pdpMinio,
		},
	}

	if err := os.MkdirAll(filepath.Dir(out.Aggregator.DBPath), 0755); err != nil {
		return app.StorageConfig{}, fmt.Errorf("creating aggregator db: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(out.Replicator.DBPath), 0755); err != nil {
		return app.StorageConfig{}, fmt.Errorf("creating replicator db: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(out.SchedulerStorage.DBPath), 0755); err != nil {
		return app.StorageConfig{}, fmt.Errorf("creating scheduler db: %w", err)
	}

	return out, nil
}
