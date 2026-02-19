package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

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

// DatabaseConfig configures the database backend.
type DatabaseConfig struct {
	// Type is the database backend: "sqlite" (default) or "postgres"
	Type     string         `mapstructure:"type" validate:"omitempty,oneof=sqlite postgres" toml:"type,omitempty"`
	Postgres PostgresConfig `mapstructure:"postgres" validate:"omitempty" toml:"postgres,omitempty"`
}

// ToAppConfig converts DatabaseConfig to app.DatabaseConfig.
func (c DatabaseConfig) ToAppConfig() (app.DatabaseConfig, error) {
	if c.Type == "postgres" {
		pgCfg, err := c.Postgres.ToAppConfig()
		if err != nil {
			return app.DatabaseConfig{}, err
		}
		return app.DatabaseConfig{
			Type:     app.DatabaseTypePostgres,
			Postgres: pgCfg,
		}, nil
	}
	return app.DatabaseConfig{
		Type: app.DatabaseTypeSQLite,
	}, nil
}

type PostgresConfig struct {
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

// ToAppConfig converts PostgresConfig to app.PostgresConfig.
// Parses the URL string and duration string into their typed equivalents.
func (c PostgresConfig) ToAppConfig() (app.PostgresConfig, error) {
	if c.URL == "" {
		return app.PostgresConfig{}, errors.New("postgres URL is required")
	}
	pgurl, err := url.Parse(c.URL)
	if err != nil {
		return app.PostgresConfig{}, fmt.Errorf("invalid postgres URL %q: %w", c.URL, err)
	}

	var connMaxLifetime time.Duration
	if c.ConnMaxLifetime != "" {
		connMaxLifetime, err = time.ParseDuration(c.ConnMaxLifetime)
		if err != nil {
			return app.PostgresConfig{}, fmt.Errorf("invalid conn_max_lifetime %q: %w", c.ConnMaxLifetime, err)
		}
	}

	return app.PostgresConfig{
		URL:             *pgurl,
		MaxOpenConns:    c.MaxOpenConns,
		MaxIdleConns:    c.MaxIdleConns,
		ConnMaxLifetime: connMaxLifetime,
	}, nil
}

type RepoConfig struct {
	DataDir  string         `mapstructure:"data_dir" validate:"required" flag:"data-dir" toml:"data_dir"`
	TempDir  string         `mapstructure:"temp_dir" validate:"required" flag:"temp-dir" toml:"temp_dir"`
	Database DatabaseConfig `mapstructure:"database" validate:"omitempty" toml:"database,omitempty"`
	S3       *S3Config      `mapstructure:"s3" validate:"omitempty" toml:"s3,omitempty"`
}

func (r RepoConfig) Validate() error {
	return validateConfig(r)
}

func (r RepoConfig) ToAppConfig() (app.StorageConfig, error) {
	dbCfg, err := r.Database.ToAppConfig()
	if err != nil {
		return app.StorageConfig{}, fmt.Errorf("database config: %w", err)
	}

	if r.DataDir == "" {
		// Return empty config for memory stores
		return app.StorageConfig{
			Database: dbCfg,
		}, nil
	}

	// Ensure directories exist
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

	return out, nil
}
