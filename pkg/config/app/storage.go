package app

// StorageConfig contains all storage paths and directories
type StorageConfig struct {
	// Root directories
	DataDir string
	TempDir string

	// Global S3 config - when set, all supported stores use S3 with separate buckets
	// named using BucketPrefix (e.g., "piri-blobs", "piri-allocations")
	S3 *S3Config

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
	Consolidation    ConsolidationStorageConfig
}

// S3Config configures S3-compatible storage (e.g., MinIO, AWS S3).
// When set on StorageConfig, all supported stores use S3 with separate buckets.
type S3Config struct {
	Endpoint     string      // API URL (e.g., "minio.example.com:9000")
	BucketPrefix string      // Prefix for bucket names (e.g., "piri-" creates piri-blobs, piri-allocations, etc.)
	Credentials  Credentials // access credentials
	Insecure     bool        // set to true to disable SSL (for development only)
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
	Dir string
}

// ConsolidationStorageConfig contains consolidation-specific storage paths
type ConsolidationStorageConfig struct {
	Dir string
}

// Credentials configures access credentials for S3-compatible storage.
type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
}

type SchedulerConfig struct {
	DBPath string
}
