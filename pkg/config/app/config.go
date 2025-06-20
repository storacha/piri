package app

import (
	"net/url"

	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/principal"
)

// Config holds the application-ready configuration with concrete types.
// This is the transformed configuration that all components (services, datastores, etc.) can use.
type Config struct {
	// Identity
	ID principal.Signer

	// The root directory for all piri state
	DataDir string
	// Temporary dir for staging data
	TempDir string

	// Service URLs and DIDs
	PublicURL          *url.URL
	UploadServiceDID   did.DID
	UploadServiceURL   *url.URL
	IndexingServiceDID did.DID
	IndexingServiceURL *url.URL

	// IPNI Publishing
	AnnounceURLs []url.URL

	// Indexing Service Proofs
	IndexingServiceProofs delegation.Proof

	// PDP Configuration
	PDPConfig *PDPConfig

	// Service Principal Mapping
	ServicePrincipalMapping map[string]string
}

// PDPConfig holds PDP-specific configuration
type PDPConfig struct {
	Endpoint *url.URL
	ProofSet uint64
}