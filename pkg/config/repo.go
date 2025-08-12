package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/samber/lo"

	"github.com/storacha/piri/pkg/config/app"
)

var DefaultRepo = Repo{
	DataDir: filepath.Join(lo.Must(os.UserHomeDir()), ".storacha"),
	TempDir: filepath.Join(os.TempDir(), "storage"),
}

type Repo struct {
	DataDir string `mapstructure:"data_dir" validate:"required" flag:"data-dir"`
	TempDir string `mapstructure:"temp_dir" validate:"required" flag:"temp-dir"`
}

func (r Repo) Validate() error {
	return validateConfig(r)
}

func (r Repo) ToAppConfig() (app.StorageConfig, error) {
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
		Allocations: app.AllocationStorageConfig{
			Dir: filepath.Join(r.DataDir, "allocation"),
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
