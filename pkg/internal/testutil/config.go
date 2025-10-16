package testutil

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/multiformats/go-multiaddr"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/principal"
	ucanhttp "github.com/storacha/go-ucanto/transport/http"
	"github.com/stretchr/testify/require"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/presets"
)

// TestConfigOption is a function that modifies a test config
type TestConfigOption func(*testing.T, *app.AppConfig)

// NewTestConfig creates a new test config with sensible defaults
// This follows the functional options pattern for easy customization
func NewTestConfig(t *testing.T, opts ...TestConfigOption) app.AppConfig {
	t.Helper()

	// Get an OS-assigned port to avoid conflicts in parallel tests
	port := GetFreePort(t)
	publicURL, err := url.Parse(fmt.Sprintf("http://localhost:%d", port))
	require.NoError(t, err)

	// Start with sensible defaults for testing
	cfg := app.AppConfig{
		Identity: app.IdentityConfig{
			Signer: testutil.Alice, // Default test signer
		},
		Server: app.ServerConfig{
			Host:      "localhost",
			Port:      uint(port),
			PublicURL: *publicURL,
		},
		Storage: app.StorageConfig{
			DataDir: "", // Empty = memory stores by default
			TempDir: "",
		},
		UCANService: app.UCANServiceConfig{
			Services: app.ExternalServicesConfig{
				PrincipalMapping: map[string]string{},
				Upload: app.UploadServiceConfig{
					Connection: testutil.Must(client.NewConnection(presets.UploadServiceDID, ucanhttp.NewChannel(presets.UploadServiceURL)))(t),
				},
				Publisher: app.PublisherServiceConfig{
					PublicMaddr:   testutil.Must(multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/http", port)))(t),
					AnnounceMaddr: testutil.Must(multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/http", port)))(t),
					AnnounceURLs:  []url.URL{}, // Empty by default for tests
				},
			},
			ProofSetID: 0,
		},
		Replicator: app.DefaultReplicatorConfig(),
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
			cfg.UCANService.Services.Upload.Connection = testutil.Must(client.NewConnection(
				did,
				ucanhttp.NewChannel(uploadURL),
			))(t)
		}
	}
}
