package proofs

import (
	"context"
	"net/http"
	"net/url"

	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/ucan"
)

// ProofService requests proofs from other UCAN enabled nodes by making
// `access/grant` invocations.
type ProofService interface {
	// Request access to be granted from the service for the passed ability.
	RequestAccess(
		ctx context.Context,
		audience ucan.Principal,
		ability ucan.Ability,
		cause invocation.Invocation,
		options ...Option,
	) (delegation.Delegation, error)
}

type requestConfig struct {
	httpClient *http.Client
	url        *url.URL
	conn       client.Connection
}

type Option func(*requestConfig)

// WithHTTPClient configures a HTTP client to use in the request.
func WithHTTPClient(h *http.Client) Option {
	return func(cfg *requestConfig) {
		cfg.httpClient = h
	}
}

// WithServiceURL configures the URL of the service to request from. If not set
// it will be inferred from the service DID, if it is a did:web.
func WithServiceURL(url *url.URL) Option {
	return func(cfg *requestConfig) {
		cfg.url = url
	}
}

// WithConnection configures the connection to use for the request. If set, the
// HTTP client and service URL options are ignored.
func WithConnection(conn client.Connection) Option {
	return func(cfg *requestConfig) {
		cfg.conn = conn
	}
}
