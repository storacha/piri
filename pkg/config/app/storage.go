package app

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
	// URL is the PostgreSQL connection string (only used when Type is "postgres").
	// Format: postgres://user:password@host:port/dbname?sslmode=disable
	URL string
}

// IsSQLite returns true if using SQLite backend (or if type is empty/default).
func (c DatabaseConfig) IsSQLite() bool {
	return c.Type == "" || c.Type == DatabaseTypeSQLite
}

// IsPostgres returns true if using PostgreSQL backend.
func (c DatabaseConfig) IsPostgres() bool {
	return c.Type == DatabaseTypePostgres
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
	Dir    string
	DBPath string
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
	Dir    string
	DBPath string
}

// AllocationStorageConfig contains allocation-specific storage paths
type AllocationStorageConfig struct {
	Dir string
}

// AcceptanceStorageConfig contains acceptance-specific storage paths
type AcceptanceStorageConfig struct {
	Dir string
}

// ReplicatorStorageConfig contains replicator-specific storage paths
type ReplicatorStorageConfig struct {
	DBPath string
}

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
}

// Credentials configures access credentials for Minio.
type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
}

type SchedulerConfig struct {
	DBPath string
}
