package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

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

// DatabaseConfig configures the database backend.
type DatabaseConfig struct {
	// Type is the database backend: "sqlite" (default) or "postgres"
	Type string `mapstructure:"type" validate:"omitempty,oneof=sqlite postgres" toml:"type,omitempty"`
	// URL is the PostgreSQL connection string (only used when type is "postgres")
	// Format: postgres://user:password@host:port/dbname?sslmode=disable
	URL string `mapstructure:"url" flag:"db-url" toml:"url,omitempty"`
	// MaxOpenConns is the maximum number of open connections to the database.
	// Only used for PostgreSQL. Default: 5
	MaxOpenConns int `mapstructure:"max_open_conns" toml:"max_open_conns,omitempty"`
	// MaxIdleConns is the maximum number of idle connections in the pool.
	// Only used for PostgreSQL. Default: 5
	MaxIdleConns int `mapstructure:"max_idle_conns" toml:"max_idle_conns,omitempty"`
	// ConnMaxLifetime is the maximum amount of time a connection may be reused.
	// Only used for PostgreSQL. Accepts Go duration strings (e.g., "30m", "1h"). Default: "30m"
	ConnMaxLifetime string `mapstructure:"conn_max_lifetime" toml:"conn_max_lifetime,omitempty"`
}

type RepoConfig struct {
	DataDir     string             `mapstructure:"data_dir" validate:"required" flag:"data-dir" toml:"data_dir"`
	TempDir     string             `mapstructure:"temp_dir" validate:"required" flag:"temp-dir" toml:"temp_dir"`
	BlobStorage *BlobStorageConfig `mapstructure:"blob_storage" validate:"omitempty" toml:"blob_storage,omitempty"`
	Database    DatabaseConfig     `mapstructure:"database" validate:"omitempty" toml:"database,omitempty"`
}

func (r RepoConfig) Validate() error {
	return validateConfig(r)
}

func (r RepoConfig) ToAppConfig() (app.StorageConfig, error) {
	// Parse connection lifetime duration if specified
	var connMaxLifetime time.Duration
	if r.Database.ConnMaxLifetime != "" {
		var err error
		connMaxLifetime, err = time.ParseDuration(r.Database.ConnMaxLifetime)
		if err != nil {
			return app.StorageConfig{}, fmt.Errorf("invalid conn_max_lifetime %q: %w", r.Database.ConnMaxLifetime, err)
		}
	}

	// Build database config to use helper methods
	dbCfg := app.DatabaseConfig{
		Type:            app.DatabaseType(r.Database.Type),
		URL:             r.Database.URL,
		MaxOpenConns:    r.Database.MaxOpenConns,
		MaxIdleConns:    r.Database.MaxIdleConns,
		ConnMaxLifetime: connMaxLifetime,
	}

	// Validate PostgreSQL requires URL
	if dbCfg.IsPostgres() && r.Database.URL == "" {
		return app.StorageConfig{}, errors.New("database URL is required when using PostgreSQL")
	}

	if r.DataDir == "" {
		// Return empty config for memory stores
		return app.StorageConfig{
			Database: dbCfg,
		}, nil
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

	// Ensure root directories exist
	if err := os.MkdirAll(r.DataDir, 0755); err != nil {
		return app.StorageConfig{}, err
	}
	if err := os.MkdirAll(r.TempDir, 0755); err != nil {
		return app.StorageConfig{}, err
	}

	// Build storage config - database paths are derived by providers, not set here
	out := app.StorageConfig{
		DataDir:  r.DataDir,
		TempDir:  r.TempDir,
		Database: dbCfg,
		Aggregator: app.AggregatorStorageConfig{
			Dir: filepath.Join(r.DataDir, "aggregator", "datastore"),
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
			Dir: filepath.Join(r.DataDir, "egress_tracker", "journal"),
		},
		Allocations: app.AllocationStorageConfig{
			Dir: filepath.Join(r.DataDir, "allocation"),
		},
		Acceptance: app.AcceptanceStorageConfig{
			Dir: filepath.Join(r.DataDir, "acceptance"),
		},
		Replicator: app.ReplicatorStorageConfig{},
		KeyStore: app.KeyStoreConfig{
			Dir: filepath.Join(r.DataDir, "wallet"),
		},
		StashStore: app.StashStoreConfig{
			Dir: filepath.Join(r.DataDir, "pdp"),
		},
		SchedulerStorage: app.SchedulerConfig{},
		PDPStore: app.PDPStoreConfig{
			Dir:   filepath.Join(r.DataDir, "pdp", "datastore"),
			Minio: pdpMinio,
		},
	}

	return out, nil
}
