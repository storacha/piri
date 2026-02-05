package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/storacha/piri/pkg/config/app"
)

// Credentials configures access credentials for S3-compatible storage.
type Credentials struct {
	AccessKeyID     string `mapstructure:"access_key_id" toml:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key" toml:"secret_access_key"`
}

// S3Config configures S3-compatible storage (e.g., MinIO, AWS S3).
// When configured, all supported stores use S3 with separate buckets
// named using the BucketPrefix (e.g., "piri-blobs", "piri-allocations").
type S3Config struct {
	Endpoint     string      `mapstructure:"endpoint" validate:"required" toml:"endpoint"`
	BucketPrefix string      `mapstructure:"bucket_prefix" validate:"required" toml:"bucket_prefix"`
	Credentials  Credentials `mapstructure:"credentials" toml:"credentials,omitempty"`
	Insecure     bool        `mapstructure:"insecure" toml:"insecure,omitempty"`
}

type RepoConfig struct {
	DataDir string    `mapstructure:"data_dir" validate:"required" flag:"data-dir" toml:"data_dir"`
	TempDir string    `mapstructure:"temp_dir" validate:"required" flag:"temp-dir" toml:"temp_dir"`
	S3      *S3Config `mapstructure:"s3" validate:"omitempty" toml:"s3,omitempty"`
}

func (r RepoConfig) Validate() error {
	return validateConfig(r)
}

func (r RepoConfig) ToAppConfig() (app.StorageConfig, error) {
	if r.DataDir == "" {
		// Return empty config for memory stores
		return app.StorageConfig{}, nil
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
			Dir: filepath.Join(r.DataDir, "pdp", "datastore"),
		},
		Consolidation: app.ConsolidationStorageConfig{
			Dir: filepath.Join(r.DataDir, "consolidation"),
		},
	}

	// Copy global S3 config if present
	if r.S3 != nil && r.S3.Endpoint != "" && r.S3.BucketPrefix != "" {
		out.S3 = &app.S3Config{
			Endpoint:     r.S3.Endpoint,
			BucketPrefix: r.S3.BucketPrefix,
			Credentials: app.Credentials{
				AccessKeyID:     r.S3.Credentials.AccessKeyID,
				SecretAccessKey: r.S3.Credentials.SecretAccessKey,
			},
			Insecure: r.S3.Insecure,
		}
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
