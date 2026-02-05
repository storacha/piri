package app

// StorageConfig contains all storage paths and directories
type StorageConfig struct {
	// Root directories
	DataDir string
	TempDir string

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
	FlatFSKeys  bool        // use FlatFS key adapter
}

// Credentials configures access credentials for Minio.
type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
}

type SchedulerConfig struct {
	DBPath string
}
