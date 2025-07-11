package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/ipni/go-libipni/maurl"
	"github.com/multiformats/go-multiaddr"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/did"
	ucanhttp "github.com/storacha/go-ucanto/transport/http"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/config/app"
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

	// Create storage config
	storageConfig := app.StorageConfig{
		DataDir: cfg.DataDir,
		TempDir: cfg.TempDir,
		Aggregator: app.AggregatorStorageConfig{
			DatastoreDir: filepath.Join(cfg.DataDir, "aggregator", "datastore"),
		},
		Blobs: app.BlobStorageConfig{
			StoreDir: filepath.Join(cfg.DataDir, "blobs"),
			TempDir:  filepath.Join(cfg.TempDir, "storage"),
		},
		Claims: app.ClaimStorageConfig{
			StoreDir: filepath.Join(cfg.DataDir, "claim"),
		},
		Publisher: app.PublisherStorageConfig{
			StoreDir: filepath.Join(cfg.DataDir, "publisher"),
		},
		Receipts: app.ReceiptStorageConfig{
			StoreDir: filepath.Join(cfg.DataDir, "receipt"),
		},
		Allocations: app.AllocationStorageConfig{
			StoreDir: filepath.Join(cfg.DataDir, "allocation"),
		},
		Replicator: app.ReplicatorStorageConfig{
			DBPath: "", // Will be set below if PDP is configured
		},
	}

	// Set replicator DB path if PDP is configured
	if cfg.PDPServerURL != "" {
		storageConfig.Replicator.DBPath = filepath.Join(cfg.DataDir, "aggregator", "jobqueue", "jobqueue.db")
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
		pieceAddr, err := multiaddr.NewMultiaddr("/http-path/" + url.PathEscape("piece/{blobCID}"))
		if err != nil {
			return app.AppConfig{}, fmt.Errorf("creating piece multiaddr: %w", err)
		}
		blobAddr = multiaddr.Join(curioAddr, pieceAddr)
	}

	servicesConfig.Publisher = app.PublisherConfig{
		PublicMaddr:   pubMaddr,
		AnnounceMaddr: pubMaddr,
		BlobMaddr:     blobAddr,
		AnnounceURLs:  announceURLs,
	}

	// Ensure directories exist
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return app.AppConfig{}, fmt.Errorf("creating data directory: %w", err)
	}
	if err := os.MkdirAll(cfg.TempDir, 0755); err != nil {
		return app.AppConfig{}, fmt.Errorf("creating temp directory: %w", err)
	}

	return app.AppConfig{
		Identity: app.IdentityConfig{
			KeyFile: cfg.KeyFile,
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