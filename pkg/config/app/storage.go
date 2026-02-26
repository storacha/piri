package app

import (
	"net/url"
	"time"
)

// DatabaseType represents the database backend type.
type DatabaseType string

const (
	// DatabaseTypeSQLite uses SQLite as the database backend (default).
	DatabaseTypeSQLite DatabaseType = "sqlite"
	// DatabaseTypePostgres uses PostgreSQL as the database backend.
	DatabaseTypePostgres DatabaseType = "postgres"
)

// DatabaseConfig contains database connection configuration.
type DatabaseConfig struct {
	// Type is the database backend type: "sqlite" (default) or "postgres".
	Type DatabaseType

	// NB: sqlite doesn't have a config.

	Postgres PostgresConfig
}

// IsSQLite returns true if using SQLite backend (or if type is empty/default).
func (c DatabaseConfig) IsSQLite() bool {
	return c.Type == "" || c.Type == DatabaseTypeSQLite
}

// IsPostgres returns true if using PostgreSQL backend.
func (c DatabaseConfig) IsPostgres() bool {
	return c.Type == DatabaseTypePostgres
}

type PostgresConfig struct {
	// URL is the PostgreSQL connection string (only used when Type is "postgres").
	// Format: postgres://user:password@host:port/dbname?sslmode=disable
	URL url.URL
	// MaxOpenConns is the maximum number of open connections to the database.
	// Only used for PostgreSQL. Zero means use default (5).
	MaxOpenConns int
	// MaxIdleConns is the maximum number of idle connections in the pool.
	// Only used for PostgreSQL. Zero means use default (5).
	MaxIdleConns int
	// ConnMaxLifetime is the maximum amount of time a connection may be reused.
	// Only used for PostgreSQL. Zero means use default (30 minutes).
	ConnMaxLifetime time.Duration
}

// StorageConfig contains all storage paths and directories
type StorageConfig struct {
	// Root directories
	DataDir string
	TempDir string

	// Database configuration (sqlite or postgres)
	Database DatabaseConfig

	// Service-specific storage subdirectories
	Aggregator       AggregatorStorageConfig
	Blobs            BlobStorageConfig
	Claims           ClaimStorageConfig
	Publisher        PublisherStorageConfig
	Receipts         ReceiptStorageConfig
	EgressTracker    EgressTrackerStorageConfig
	Allocations      AllocationStorageConfig
	Acceptance       AcceptanceStorageConfig
	Replicator       ReplicatorStorageConfig
	KeyStore         KeyStoreConfig
	StashStore       StashStoreConfig
	SchedulerStorage SchedulerConfig
	PDPStore         PDPStoreConfig
}

// AggregatorStorageConfig contains aggregator-specific storage paths
type AggregatorStorageConfig struct {
	Dir string
}

// BlobStorageConfig contains blob-specific storage paths
type BlobStorageConfig struct {
	Dir    string
	TmpDir string
}

// ClaimStorageConfig contains claim-specific storage paths
type ClaimStorageConfig struct {
	Dir string
}

// PublisherStorageConfig contains publisher-specific storage paths
type PublisherStorageConfig struct {
	Dir string
}

// ReceiptStorageConfig contains receipt-specific storage paths
type ReceiptStorageConfig struct {
	Dir string
}

// EgressTrackerStorageConfig contains egress tracker store-specific storage paths
type EgressTrackerStorageConfig struct {
	Dir string
}

// AllocationStorageConfig contains allocation-specific storage paths
type AllocationStorageConfig struct {
	Dir string
}

// AcceptanceStorageConfig contains acceptance-specific storage paths
type AcceptanceStorageConfig struct {
	Dir string
}

// ReplicatorStorageConfig contains replicator-specific storage paths.
// Currently empty - SQLite paths are derived by providers.
type ReplicatorStorageConfig struct{}

type KeyStoreConfig struct {
	Dir string
}

type StashStoreConfig struct {
	Dir string
}

type PDPStoreConfig struct {
	Dir   string
	Minio MinioConfig
}

// MinioConfig configures Minio - an S3 compatible object store.
type MinioConfig struct {
	Endpoint    string      // API URL
	Bucket      string      // bucket name
	Credentials Credentials // access credentials
	Insecure    bool        // set to true to disable SSL
	FlatFSKeys  bool        // use FlatFS key adapter
}

// Credentials configures access credentials for Minio.
type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
}

// SchedulerConfig contains scheduler-specific storage paths.
// Currently empty - SQLite paths are derived by providers.
type SchedulerConfig struct{}
