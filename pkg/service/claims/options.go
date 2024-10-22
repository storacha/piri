package claims

import (
	"net/url"

	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/transport/http"
	"github.com/storacha/go-ucanto/ucan"
)

type options struct {
	announceURLs          []url.URL
	indexingService       client.Connection
	indexingServiceProofs delegation.Proofs
}

type Option func(*options) error

// WithPublisherDirectAnnounce sets indexer URLs to send direct HTTP
// announcements to.
func WithPublisherDirectAnnounce(announceURLs ...url.URL) Option {
	return func(o *options) error {
		o.announceURLs = append(o.announceURLs, announceURLs...)
		return nil
	}
}

// WithPublisherIndexingService sets the client connection to the indexing UCAN
// service.
func WithPublisherIndexingService(conn client.Connection) Option {
	return func(opts *options) error {
		opts.indexingService = conn
		return nil
	}
}

// WithPublisherIndexingServiceConfig configures UCAN service invocation details
// for communicating with the indexing service.
func WithPublisherIndexingServiceConfig(serviceDID ucan.Principal, serviceURL url.URL) Option {
	return func(opts *options) error {
		channel := http.NewHTTPChannel(&serviceURL)
		conn, err := client.NewConnection(serviceDID, channel)
		if err != nil {
			return err
		}
		opts.indexingService = conn
		return nil
	}
}

// WithPublisherIndexingServiceProof configures proofs for UCAN invocations to
// the indexing service.
func WithPublisherIndexingServiceProof(proof ...delegation.Proof) Option {
	return func(opts *options) error {
		opts.indexingServiceProofs = proof
		return nil
	}
}

// WithLogLevel changes the log level for the claims subsystem.
func WithLogLevel(level string) Option {
	return func(c *options) error {
		logging.SetLogLevel("claims", level)
		return nil
	}
}
