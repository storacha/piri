package testutil

import (
	"net/url"
	"testing"

	"github.com/multiformats/go-multiaddr"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/principal"
	ucanhttp "github.com/storacha/go-ucanto/transport/http"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/presets"
)

// TestConfigOption is a function that modifies a test config
type TestConfigOption func(*testing.T, *app.AppConfig)

// NewTestConfig creates a new test config with sensible defaults
// This follows the functional options pattern for easy customization
func NewTestConfig(t *testing.T, opts ...TestConfigOption) app.AppConfig {
	t.Helper()

	// Start with sensible defaults for testing
	cfg := app.AppConfig{
		Identity: app.IdentityConfig{
			Signer: testutil.Alice, // Default test signer
		},
		Server: app.ServerConfig{
			Host:      "localhost",
			Port:      8080,
			PublicURL: testutil.Must(url.Parse("http://localhost:8080"))(t),
		},
		Storage: app.StorageConfig{
			DataDir: "", // Empty = memory stores by default
			TempDir: "",
		},
		Services: app.ServicesConfig{
			UploadService: app.ServiceConnectionConfig{
				Connection: testutil.Must(client.NewConnection(presets.UploadServiceDID, ucanhttp.NewHTTPChannel(presets.UploadServiceURL)))(t),
			},
			Publisher: app.PublisherConfig{
				PublicMaddr:   testutil.Must(multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/8080/http"))(t),
				AnnounceMaddr: testutil.Must(multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/8080/http"))(t),
				AnnounceURLs:  []url.URL{}, // Empty by default for tests
			},
			ServicePrincipalMapping: map[string]string{}, // Empty by default
		},
	}

	// Apply all options
	for _, opt := range opts {
		opt(t, &cfg)
	}

	return cfg
}

// WithSigner sets the identity signer
func WithSigner(signer principal.Signer) TestConfigOption {
	return func(_ *testing.T, cfg *app.AppConfig) {
		cfg.Identity.Signer = signer
	}
}

// WithUploadServiceURL sets the upload service URL
func WithUploadServiceURL(uploadURL *url.URL) TestConfigOption {
	return func(t *testing.T, cfg *app.AppConfig) {
		if uploadURL != nil {
			// Create a new connection with the provided URL
			did := presets.UploadServiceDID
			if did.String() == "" {
				// Use Alice as a fallback for tests
				did = testutil.Alice.DID()
			}
			cfg.Services.UploadService.Connection = testutil.Must(client.NewConnection(
				did,
				ucanhttp.NewHTTPChannel(uploadURL),
			))(t)
		}
	}
}
