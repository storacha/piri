package app

import (
	"net/url"

	"github.com/multiformats/go-multiaddr"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/principal"
)

// AppConfig is the root configuration for the entire application
type AppConfig struct {
	// Identity configuration
	Identity IdentityConfig

	// Server configuration
	Server ServerConfig

	// Storage paths and directories
	Storage StorageConfig

	// External service connections
	External ExternalServicesConfig

	// Service-specific configurations
	Services ServicesConfig
}

// IdentityConfig contains identity-related configuration
type IdentityConfig struct {
	// The principal signer for this service
	Signer principal.Signer
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Host      string
	Port      uint
	PublicURL *url.URL
}

// StorageConfig contains all storage paths and directories
type StorageConfig struct {
	// Root directories
	DataDir string
	TempDir string

	// Service-specific storage subdirectories
	Aggregator  AggregatorStorageConfig
	Blobs       BlobStorageConfig
	Claims      ClaimStorageConfig
	Publisher   PublisherStorageConfig
	Receipts    ReceiptStorageConfig
	Allocations AllocationStorageConfig
	Replicator  ReplicatorStorageConfig
}

// AggregatorStorageConfig contains aggregator-specific storage paths
type AggregatorStorageConfig struct {
	DatastoreDir string
}

// BlobStorageConfig contains blob-specific storage paths
type BlobStorageConfig struct {
	StoreDir string
	TempDir  string
}

// ClaimStorageConfig contains claim-specific storage paths
type ClaimStorageConfig struct {
	StoreDir string
}

// PublisherStorageConfig contains publisher-specific storage paths
type PublisherStorageConfig struct {
	StoreDir string
}

// ReceiptStorageConfig contains receipt-specific storage paths
type ReceiptStorageConfig struct {
	StoreDir string
}

// AllocationStorageConfig contains allocation-specific storage paths
type AllocationStorageConfig struct {
	StoreDir string
}

// ReplicatorStorageConfig contains replicator-specific storage paths
type ReplicatorStorageConfig struct {
	DBPath string
}

// ExternalServicesConfig contains all external service connections
type ExternalServicesConfig struct {
	UploadService   ServiceConnectionConfig
	IndexingService IndexingServiceConfig
	PDPServer       *PDPServerConfig // Pointer because it's optional
}

// ServiceConnectionConfig contains basic service connection details
type ServiceConnectionConfig struct {
	Connection client.Connection
}

// IndexingServiceConfig contains indexing service connection and auth details
type IndexingServiceConfig struct {
	Connection client.Connection
	Proofs     delegation.Proofs
}

// PDPServerConfig contains PDP server connection details
type PDPServerConfig struct {
	URL      *url.URL
	ProofSet uint64
}

// ServicesConfig contains service-specific configurations
type ServicesConfig struct {
	Publisher PublisherConfig
	// Service DID to principal DID mapping for authentication
	ServicePrincipalMapping map[string]string
	// Future: Replicator, Blobs, Claims configs can be added here
}

// PublisherConfig contains publisher service configuration
type PublisherConfig struct {
	// The public facing multiaddr of the publisher
	PublicMaddr multiaddr.Multiaddr
	// The address put into announce messages to tell indexers where to fetch advertisements from
	AnnounceMaddr multiaddr.Multiaddr
	// Address to tell indexers where to fetch blobs from
	BlobMaddr multiaddr.Multiaddr
	// Indexer URLs to send direct HTTP announcements to
	AnnounceURLs []url.URL
}
