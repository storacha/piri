package app

import (
	"net/url"

	"github.com/multiformats/go-multiaddr"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
)

type PiriConfig struct {
	Repo RepoConfig

	PublicURL *url.URL

	UploadServiceConnection client.Connection `name:"upload_service_connection"`

	Publisher PublisherServiceConfig
}

type PublisherServiceConfig struct {
	// the public facing mulitadder of the publisher
	PublicMaddr multiaddr.Multiaddr
	// the address put into announce messages to tell indexers where to fetch advertisements from.
	AnnounceMaddr multiaddr.Multiaddr
	// address to tell indexers where to fetch blobs from.
	BlobMaddr multiaddr.Multiaddr
	// indexer URLs to send direct HTTP announcements to.
	AnnounceURLs []url.URL
	// the client connection to the indexing UCAN service.
	IndexingService client.Connection
	// proofs for UCAN invocations to the indexing service.
	IndexingServiceProofs delegation.Proofs
}

type RepoConfig struct {
	// Root directory all stores are under
	DataDir string

	AggregatorStoreDir string

	AllocationStoreDir string

	BlobStoreDir    string
	BlobStoreTmpDir string

	ClaimStoreDir string

	PublisherStoreDir string

	ReceiptStoreDir string
}
