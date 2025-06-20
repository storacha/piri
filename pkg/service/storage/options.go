package storage

import (
	"database/sql"
	"net/url"

	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	"github.com/multiformats/go-multiaddr"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/transport/http"
	"github.com/storacha/go-ucanto/ucan"

	"github.com/storacha/piri/pkg/access"
	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/presigner"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/store/claimstore"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

type PDPConfig struct {
	PDPService   pdp.PDP
	PDPDatastore datastore.Datastore
	PDPServerURL *url.URL
	ProofSet     uint64
	DatabasePath string
}

type config struct {
	id                    principal.Signer
	publicURL             url.URL
	blobsPublicURL        url.URL
	blobsPresigner        presigner.RequestPresigner
	blobStore             blobstore.Blobstore
	blobsAccess           access.Access
	allocationStore       allocationstore.AllocationStore
	allocationDatastore   datastore.Datastore
	claimStore            claimstore.ClaimStore
	claimDatastore        datastore.Datastore
	publisherStore        store.PublisherStore
	publisherDatastore    datastore.Datastore
	publisherAnnouceAddr  multiaddr.Multiaddr
	publisherBlobAddress  multiaddr.Multiaddr
	receiptStore          receiptstore.ReceiptStore
	receiptDatastore      datastore.Datastore
	pdp                   *PDPConfig
	announceURLs          []url.URL
	indexingService       client.Connection
	indexingServiceProofs delegation.Proofs
	uploadService         client.Connection
	replicatorDB          *sql.DB
}

type Option func(*config) error

// WithIdentity configures the storage service identity, used to sign UCAN
// invocations and receipts.
func WithIdentity(signer principal.Signer) Option {
	return func(c *config) error {
		c.id = signer
		return nil
	}
}

// WithPublicURL configures the URL this storage node will be publically
// accessible from.
func WithPublicURL(url url.URL) Option {
	return func(c *config) error {
		c.publicURL = url
		return nil
	}
}

// WithBlobstore configures the blob storage to use.
func WithBlobstore(blobStore blobstore.Blobstore) Option {
	return func(c *config) error {
		c.blobStore = blobStore
		return nil
	}
}

// WithBlobsPublicURL configures the blob storage to use a public URL
func WithBlobsPublicURL(blobStorePublicURL url.URL) Option {
	return func(c *config) error {
		c.blobsPublicURL = blobStorePublicURL
		return nil
	}
}

// WithBlobsAccess configures the access instance for blob storage.
func WithBlobsAccess(access access.Access) Option {
	return func(c *config) error {
		c.blobsAccess = access
		return nil
	}
}

// WithBlobsPresigner configures the blob storage to use a set presigner
func WithBlobsPresigner(blobStorePresigner presigner.RequestPresigner) Option {
	return func(c *config) error {
		c.blobsPresigner = blobStorePresigner
		return nil
	}
}

// WithAllocationStore configures the allocation store directly
func WithAllocationStore(allocationStore allocationstore.AllocationStore) Option {
	return func(c *config) error {
		c.allocationStore = allocationStore
		return nil
	}
}

// WithAllocationDatastore configures the underlying datastore to use for
// storing allocation records. Note: the datastore MUST have efficient support
// for prefix queries.
func WithAllocationDatastore(dstore datastore.Datastore) Option {
	return func(c *config) error {
		c.allocationDatastore = dstore
		return nil
	}
}

// WithClaimStore configures the store for content claims directly
func WithClaimStore(claimStore claimstore.ClaimStore) Option {
	return func(c *config) error {
		c.claimStore = claimStore
		return nil
	}
}

// WithClaimDatastore configures the underlying datastore to use for storing
// content claims made by this node.
func WithClaimDatastore(dstore datastore.Datastore) Option {
	return func(c *config) error {
		c.claimDatastore = dstore
		return nil
	}
}

// WithReceiptStore configures the store for receipts directly
func WithReceiptStore(receiptStore receiptstore.ReceiptStore) Option {
	return func(c *config) error {
		c.receiptStore = receiptStore
		return nil
	}
}

// WithReceiptDatastore configures the underlying datastore for use storing receipts
// made for this node
func WithReceiptDatastore(dstore datastore.Datastore) Option {
	return func(c *config) error {
		c.receiptDatastore = dstore
		return nil
	}
}

// WithPublisherStore configures the store for IPNI advertisements and their
// entries directly.
func WithPublisherStore(publisherStore store.PublisherStore) Option {
	return func(c *config) error {
		c.publisherStore = publisherStore
		return nil
	}
}

// WithPublisherDatastore configures the underlying datastore to use for storing
// IPNI advertisements and their entries.
func WithPublisherDatastore(dstore datastore.Datastore) Option {
	return func(c *config) error {
		c.publisherDatastore = dstore
		return nil
	}
}

// WithPublisherAnnounceAddress sets the address put into announce messages to
// tell indexers where to fetch advertisements from.
func WithPublisherAnnounceAddress(addr multiaddr.Multiaddr) Option {
	return func(c *config) error {
		c.publisherAnnouceAddr = addr
		return nil
	}
}

// WithPublisherBlobAddress sets the multiaddr for blobs used by the publisher
func WithPublisherBlobAddress(addr multiaddr.Multiaddr) Option {
	return func(c *config) error {
		c.publisherBlobAddress = addr
		return nil
	}
}

// WithPublisherDirectAnnounce sets IPNI node URLs to send direct HTTP
// announcements to.
func WithPublisherDirectAnnounce(announceURLs ...url.URL) Option {
	return func(c *config) error {
		c.announceURLs = append(c.announceURLs, announceURLs...)
		return nil
	}
}

// WithPublisherIndexingService sets the client connection to the indexing UCAN
// service.
func WithPublisherIndexingService(conn client.Connection) Option {
	return func(c *config) error {
		c.indexingService = conn
		return nil
	}
}

// WithPublisherIndexingServiceConfig configures UCAN service invocation details
// for communicating with the indexing service.
func WithPublisherIndexingServiceConfig(serviceDID ucan.Principal, serviceURL url.URL) Option {
	return func(c *config) error {
		channel := http.NewHTTPChannel(&serviceURL)
		conn, err := client.NewConnection(serviceDID, channel)
		if err != nil {
			return err
		}
		c.indexingService = conn
		return nil
	}
}

// WithUploadServiceConfig configures UCAN service invocation details
// for communicating with the upload service.
func WithUploadServiceConfig(serviceDID ucan.Principal, serviceURL url.URL) Option {
	return func(c *config) error {
		channel := http.NewHTTPChannel(&serviceURL)
		conn, err := client.NewConnection(serviceDID, channel)
		if err != nil {
			return err
		}
		c.uploadService = conn
		return nil
	}
}

// WithPublisherIndexingServiceProof configures proofs for UCAN invocations to
// the indexing service.
func WithPublisherIndexingServiceProof(proof ...delegation.Proof) Option {
	return func(c *config) error {
		c.indexingServiceProofs = proof
		return nil
	}
}

// WithLogLevel changes the log level of a specific subsystem name=="*" changes
// all subsystems.
func WithLogLevel(name string, level string) Option {
	return func(c *config) error {
		logging.SetLogLevel(name, level)
		return nil
	}
}

// WithPDPConfig causes the service to run through Curio and do PDP proofs
func WithPDPConfig(pdpConfig PDPConfig) Option {
	return func(c *config) error {
		c.pdp = &pdpConfig
		return nil
	}
}

func WithReplicatorDB(db *sql.DB) Option {
	return func(c *config) error {
		c.replicatorDB = db
		return nil
	}
}
