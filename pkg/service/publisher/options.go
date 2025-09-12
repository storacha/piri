package publisher

import (
	"net/url"

	logging "github.com/ipfs/go-log/v2"
	"github.com/multiformats/go-multiaddr"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/transport/http"
	"github.com/storacha/go-ucanto/ucan"
)

type options struct {
	blobAddr              multiaddr.Multiaddr
	announceAddr          multiaddr.Multiaddr
	announceURLs          []url.URL
	indexingService       client.Connection
	indexingServiceProofs delegation.Proofs
}

type Option func(*options) error

// WithAnnounceAddress sets the address put into announce messages to tell
// indexers where to fetch advertisements from.
func WithAnnounceAddress(addr multiaddr.Multiaddr) Option {
	return func(o *options) error {
		o.announceAddr = addr
		return nil
	}
}

// WithBlobAddress sets a custom address to tell indexers where to fetch blobs from
func WithBlobAddress(addr multiaddr.Multiaddr) Option {
	return func(o *options) error {
		o.blobAddr = addr
		return nil
	}
}

// WithDirectAnnounce sets indexer URLs to send direct HTTP announcements to.
func WithDirectAnnounce(announceURLs ...url.URL) Option {
	return func(o *options) error {
		o.announceURLs = append(o.announceURLs, announceURLs...)
		return nil
	}
}

// WithIndexingService sets the client connection to the indexing UCAN service.
func WithIndexingService(conn client.Connection) Option {
	return func(opts *options) error {
		opts.indexingService = conn
		return nil
	}
}

// WithIndexingServiceConfig configures UCAN service invocation details for
// communicating with the indexing service.
func WithIndexingServiceConfig(serviceDID ucan.Principal, serviceURL url.URL) Option {
	return func(opts *options) error {
		channel := http.NewChannel(&serviceURL)
		conn, err := client.NewConnection(serviceDID, channel)
		if err != nil {
			return err
		}
		opts.indexingService = conn
		return nil
	}
}

// WithIndexingServiceProof configures proofs for UCAN invocations to the
// indexing service.
func WithIndexingServiceProof(proof ...delegation.Proof) Option {
	return func(opts *options) error {
		opts.indexingServiceProofs = proof
		return nil
	}
}

// WithLogLevel changes the log level for the publisher subsystem.
func WithLogLevel(level string) Option {
	return func(c *options) error {
		logging.SetLogLevel("publisher", level)
		return nil
	}
}
