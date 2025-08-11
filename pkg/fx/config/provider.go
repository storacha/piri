package config

import (
	crypto_ed25519 "crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"

	"github.com/ipni/go-libipni/maurl"
	"github.com/multiformats/go-multiaddr"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/principal"
	ed25519 "github.com/storacha/go-ucanto/principal/ed25519/signer"
	ucanhttp "github.com/storacha/go-ucanto/transport/http"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/mahttp"
)

var Module = fx.Module("config",
	fx.Provide(
		ProvideAppConfig,
		// Provide named values for services that still need them
		fx.Annotate(
			ProvideUploadServiceConnection,
			fx.ResultTags(`name:"upload_service_connection"`),
		),
	),
)

// ProvideAppConfig provides the new organized application configuration
func ProvideAppConfig(cfg config.UCANServer) (app.AppConfig, error) {
	// Parse public URL
	var publicURL *url.URL
	var err error
	if cfg.PublicURL != "" {
		publicURL, err = url.Parse(cfg.PublicURL)
		if err != nil {
			return app.AppConfig{}, fmt.Errorf("parsing public URL: %w", err)
		}
	} else {
		publicURL, err = url.Parse(fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port))
		if err != nil {
			return app.AppConfig{}, fmt.Errorf("creating default public URL: %w", err)
		}
	}

	// Create server config
	serverConfig := app.ServerConfig{
		Host:      cfg.Host,
		Port:      cfg.Port,
		PublicURL: publicURL,
	}

	// Initialize storage config
	var storageConfig app.StorageConfig

	if cfg.Repo.DataDir != "" {
		// Ensure directories exist
		if err := os.MkdirAll(cfg.Repo.DataDir, 0755); err != nil {
			return app.AppConfig{}, fmt.Errorf("creating data directory: %w", err)
		}
		if err := os.MkdirAll(cfg.Repo.TempDir, 0755); err != nil {
			return app.AppConfig{}, fmt.Errorf("creating temp directory: %w", err)
		}

		// Create storage config with file-based paths
		storageConfig = app.StorageConfig{
			DataDir: cfg.Repo.DataDir,
			TempDir: cfg.Repo.TempDir,
			Aggregator: app.AggregatorStorageConfig{
				DatastoreDir: filepath.Join(cfg.Repo.DataDir, "aggregator", "datastore"),
			},
			Blobs: app.BlobStorageConfig{
				StoreDir: filepath.Join(cfg.Repo.DataDir, "blobs"),
				TempDir:  filepath.Join(cfg.Repo.TempDir, "storage"),
			},
			Claims: app.ClaimStorageConfig{
				StoreDir: filepath.Join(cfg.Repo.DataDir, "claim"),
			},
			Publisher: app.PublisherStorageConfig{
				StoreDir: filepath.Join(cfg.Repo.DataDir, "publisher"),
			},
			Receipts: app.ReceiptStorageConfig{
				StoreDir: filepath.Join(cfg.Repo.DataDir, "receipt"),
			},
			Allocations: app.AllocationStorageConfig{
				StoreDir: filepath.Join(cfg.Repo.DataDir, "allocation"),
			},
			Replicator: app.ReplicatorStorageConfig{
				DBPath: "", // Will be set below if PDP is configured
			},
		}

		// Set replicator DB path if PDP is configured
		if cfg.PDPServerURL != "" {
			storageConfig.Replicator.DBPath = filepath.Join(cfg.Repo.DataDir, "aggregator", "jobqueue", "jobqueue.db")
		}
	} else {
		// Empty storage config for memory stores
		storageConfig = app.StorageConfig{
			DataDir: "",
			TempDir: "",
		}
	}

	// Create external services config
	var externalConfig app.ExternalServicesConfig

	// Upload service
	uploadDID, err := did.Parse(cfg.UploadServiceDID)
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("parsing upload service DID: %w", err)
	}
	uploadURL, err := url.Parse(cfg.UploadServiceURL)
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("parsing upload service URL: %w", err)
	}
	uploadChannel := ucanhttp.NewHTTPChannel(uploadURL)
	uploadConn, err := client.NewConnection(uploadDID, uploadChannel)
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("creating upload service connection: %w", err)
	}
	externalConfig.UploadService = app.ServiceConnectionConfig{
		Connection: uploadConn,
	}

	// Indexing service
	indexingDID, err := did.Parse(cfg.IndexingServiceDID)
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("parsing indexing service DID: %w", err)
	}
	indexingURL, err := url.Parse(cfg.IndexingServiceURL)
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("parsing indexing service URL: %w", err)
	}
	indexingChannel := ucanhttp.NewHTTPChannel(indexingURL)
	indexingConn, err := client.NewConnection(indexingDID, indexingChannel)
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("creating indexing service connection: %w", err)
	}

	// Parse indexing service proofs if provided
	var indexingProofs delegation.Proofs
	if cfg.IndexingServiceProof != "" {
		dlg, err := delegation.Parse(cfg.IndexingServiceProof)
		if err != nil {
			return app.AppConfig{}, fmt.Errorf("parsing indexing service proof: %w", err)
		}
		indexingProofs = delegation.Proofs{delegation.FromDelegation(dlg)}
	}

	externalConfig.IndexingService = app.IndexingServiceConfig{
		Connection: indexingConn,
		Proofs:     indexingProofs,
	}

	// PDP server (optional)
	if cfg.PDPServerURL != "" {
		pdpURL, err := url.Parse(cfg.PDPServerURL)
		if err != nil {
			return app.AppConfig{}, fmt.Errorf("parsing PDP server URL: %w", err)
		}
		externalConfig.PDPServer = &app.PDPServerConfig{
			URL:      pdpURL,
			ProofSet: cfg.ProofSet,
		}
	}

	// Create services config
	servicesConfig := app.ServicesConfig{
		Publisher: app.PublisherConfig{
			// Will be populated below
		},
		ServicePrincipalMapping: cfg.ServicePrincipalMapping,
	}

	// Build publisher config
	pubMaddr, err := maurl.FromURL(publicURL)
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("converting public URL to multiaddr: %w", err)
	}

	// Parse IPNI announce URLs
	var announceURLs []url.URL
	for _, s := range cfg.IPNIAnnounceURLs {
		u, err := url.Parse(s)
		if err != nil {
			return app.AppConfig{}, fmt.Errorf("parsing IPNI announce URL %s: %w", s, err)
		}
		announceURLs = append(announceURLs, *u)
	}

	// Handle blob address for PDP
	var blobAddr multiaddr.Multiaddr
	if cfg.PDPServerURL != "" {
		pdpURL, err := url.Parse(cfg.PDPServerURL)
		if err != nil {
			return app.AppConfig{}, fmt.Errorf("parsing PDP server URL: %w", err)
		}
		curioAddr, err := maurl.FromURL(pdpURL)
		if err != nil {
			return app.AppConfig{}, fmt.Errorf("converting PDP URL to multiaddr: %w", err)
		}
		blobAddr, err = mahttp.JoinPath(curioAddr, "piece/{blobCID}")
		if err != nil {
			return app.AppConfig{}, fmt.Errorf("joining blob path to PDP multiaddr: %w", err)
		}
	}

	servicesConfig.Publisher = app.PublisherConfig{
		PublicMaddr:   pubMaddr,
		AnnounceMaddr: pubMaddr,
		BlobMaddr:     blobAddr,
		AnnounceURLs:  announceURLs,
	}

	identity, err := ReadPrivateKeyFromPEM(cfg.KeyFile)
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("failed to read private key from PEM file: %w", err)
	}

	return app.AppConfig{
		Identity: app.IdentityConfig{
			Signer: identity,
		},
		Server:   serverConfig,
		Storage:  storageConfig,
		External: externalConfig,
		Services: servicesConfig,
	}, nil
}

// ProvideUploadServiceConnection provides the upload service connection for backward compatibility
func ProvideUploadServiceConnection(cfg app.AppConfig) client.Connection {
	return cfg.External.UploadService.Connection
}

func ReadPrivateKeyFromPEM(path string) (principal.Signer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	pemData, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("reading private key: %w", err)
	}

	var privateKey *crypto_ed25519.PrivateKey
	rest := pemData

	// Loop until no more blocks
	for {
		block, remaining := pem.Decode(rest)
		if block == nil {
			// No more PEM blocks
			break
		}
		rest = remaining

		// Look for "PRIVATE KEY"
		if block.Type == "PRIVATE KEY" {
			parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("failed to parse PKCS#8 private key: %w", err)
			}

			// We expect a ed25519 private key, cast it
			key, ok := parsedKey.(crypto_ed25519.PrivateKey)
			if !ok {
				return nil, fmt.Errorf("the parsed key is not an ED25519 private key")
			}
			privateKey = &key
			break
		}
	}

	if privateKey == nil {
		return nil, fmt.Errorf("could not find a PRIVATE KEY block in the PEM file")
	}
	return ed25519.FromRaw(*privateKey)
}
