package serve

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/ipni/go-libipni/maurl"
	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/principal"
	ucanhttp "github.com/storacha/go-ucanto/transport/http"
	"go.uber.org/fx"

	"github.com/storacha/piri/cmd/cliutil"
	"github.com/storacha/piri/pkg/config"
	configapp "github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/fx/app"
	"github.com/storacha/piri/pkg/mahttp"
)

var (
	FullCmd = &cobra.Command{
		Use:    "full",
		Short:  "Start the full server.",
		Args:   cobra.NoArgs,
		RunE:   startFullServer,
		Hidden: true,
	}
)

func init() {
	FullCmd.Flags().String(
		"host",
		config.DefaultUCANServer.Host,
		"Host to listen on")
	cobra.CheckErr(viper.BindPFlag("host", FullCmd.Flags().Lookup("host")))

	FullCmd.Flags().Uint(
		"port",
		config.DefaultUCANServer.Port,
		"Port to listen on",
	)
	cobra.CheckErr(viper.BindPFlag("port", FullCmd.Flags().Lookup("port")))

	FullCmd.Flags().String(
		"public-url",
		config.DefaultUCANServer.PublicURL,
		"URL the node is publicly accessible at and exposed to other storacha services",
	)
	cobra.CheckErr(viper.BindPFlag("public_url", FullCmd.Flags().Lookup("public-url")))

	FullCmd.Flags().String(
		"pdp-server-url",
		config.DefaultUCANServer.PDPServerURL,
		"URL used to connect to pdp server",
	)
	cobra.CheckErr(viper.BindPFlag("pdp_server_url", FullCmd.Flags().Lookup("pdp-server-url")))

	FullCmd.Flags().Uint64(
		"proof-set",
		config.DefaultUCANServer.ProofSet,
		"Proofset to use with PDP",
	)
	cobra.CheckErr(viper.BindPFlag("proof_set", FullCmd.Flags().Lookup("proof-set")))

	FullCmd.Flags().String(
		"indexing-service-proof",
		config.DefaultUCANServer.IndexingServiceProof,
		"A delegation that allows the node to cache claims with the indexing service",
	)
	cobra.CheckErr(viper.BindPFlag("indexing_service_proof", FullCmd.Flags().Lookup("indexing-service-proof")))

	FullCmd.Flags().String(
		"indexing-service-did",
		config.DefaultUCANServer.IndexingServiceDID,
		"DID of the indexing service",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("indexing-service-did"))
	cobra.CheckErr(viper.BindPFlag("indexing_service_did", FullCmd.Flags().Lookup("indexing-service-did")))

	FullCmd.Flags().String(
		"indexing-service-url",
		config.DefaultUCANServer.IndexingServiceURL,
		"URL of the indexing service",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("indexing-service-url"))
	cobra.CheckErr(viper.BindPFlag("indexing_service_url", FullCmd.Flags().Lookup("indexing-service-url")))

	FullCmd.Flags().String(
		"upload-service-did",
		config.DefaultUCANServer.UploadServiceDID,
		"DID of the upload service",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("upload-service-did"))
	cobra.CheckErr(viper.BindPFlag("upload_service_did", FullCmd.Flags().Lookup("upload-service-did")))

	FullCmd.Flags().String(
		"upload-service-url",
		config.DefaultUCANServer.UploadServiceURL,
		"URL of the upload service",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("upload-service-url"))
	cobra.CheckErr(viper.BindPFlag("upload_service_url", FullCmd.Flags().Lookup("upload-service-url")))

	FullCmd.Flags().StringSlice(
		"ipni-announce-urls",
		config.DefaultUCANServer.IPNIAnnounceURLs,
		"A list of IPNI announce URLs")
	cobra.CheckErr(FullCmd.Flags().MarkHidden("ipni-announce-urls"))
	cobra.CheckErr(viper.BindPFlag("ipni_announce_urls", FullCmd.Flags().Lookup("ipni-announce-urls")))

	FullCmd.Flags().StringToString(
		"service-principal-mapping",
		config.DefaultUCANServer.ServicePrincipalMapping,
		"Mapping of service DIDs to principal DIDs",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("service-principal-mapping"))
	cobra.CheckErr(viper.BindPFlag("service_principal_mapping", FullCmd.Flags().Lookup("service-principal-mapping")))

}

func startFullServer(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	// Load and validate configuration
	cfg, err := config.Load[config.UCANServer]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if cfg.PDPServerURL != "" && cfg.ProofSet == 0 {
		return fmt.Errorf("must set --proof-set when using --pdp-server-url")
	}
	if cfg.ProofSet != 0 && cfg.PDPServerURL == "" {
		return fmt.Errorf("must set --pdp-server-url when using --proofset")
	}

	// Transform CLI config to app config
	appConfig, err := transformToAppConfig(cfg)
	if err != nil {
		return fmt.Errorf("transforming config: %w", err)
	}

	// Create the fx application with the app config
	fxApp := fx.New(
		fx.Supply(appConfig),
		app.FullModule(appConfig),
		fx.Invoke(
			// Print hero after startup
			printHero,
		),
	)

	// Start the application
	if err := fxApp.Start(ctx); err != nil {
		return fmt.Errorf("starting fx app: %w", err)
	}

	// Wait for interrupt signal
	<-fxApp.Done()

	// Stop the application
	stopCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := fxApp.Stop(stopCtx); err != nil {
		return fmt.Errorf("stopping fx app: %w", err)
	}

	return nil
}

// printHero prints the hero banner after startup
func printHero(id principal.Signer) {
	go func() {
		time.Sleep(time.Millisecond * 50)
		cliutil.PrintHero(id.DID())
	}()
}

// transformToAppConfig transforms CLI configuration to application configuration
func transformToAppConfig(cfg config.UCANServer) (configapp.AppConfig, error) {
	// Load the principal signer
	signer, err := cliutil.ReadPrivateKeyFromPEM(cfg.KeyFile)
	if err != nil {
		return configapp.AppConfig{}, fmt.Errorf("loading principal signer: %w", err)
	}

	// Parse public URL
	var publicURL *url.URL
	if cfg.PublicURL != "" {
		publicURL, err = url.Parse(cfg.PublicURL)
		if err != nil {
			return configapp.AppConfig{}, fmt.Errorf("parsing public URL: %w", err)
		}
	} else {
		publicURL, err = url.Parse(fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port))
		if err != nil {
			return configapp.AppConfig{}, fmt.Errorf("creating default public URL: %w", err)
		}
	}

	// Build storage config
	storageConfig := buildStorageConfig(cfg)

	// Build external services config
	externalConfig, err := buildExternalServicesConfig(cfg)
	if err != nil {
		return configapp.AppConfig{}, fmt.Errorf("building external services config: %w", err)
	}

	// Build services config
	servicesConfig, err := buildServicesConfig(cfg, publicURL)
	if err != nil {
		return configapp.AppConfig{}, fmt.Errorf("building services config: %w", err)
	}

	return configapp.AppConfig{
		Identity: configapp.IdentityConfig{
			Signer: signer,
		},
		Server: configapp.ServerConfig{
			Host:      cfg.Host,
			Port:      cfg.Port,
			PublicURL: publicURL,
		},
		Storage:  storageConfig,
		External: externalConfig,
		Services: servicesConfig,
	}, nil
}

func buildStorageConfig(cfg config.UCANServer) configapp.StorageConfig {
	if cfg.Repo.DataDir == "" {
		// Return empty config for memory stores
		return configapp.StorageConfig{
			DataDir: "",
			TempDir: "",
		}
	}

	// Ensure directories exist
	os.MkdirAll(cfg.Repo.DataDir, 0755)
	os.MkdirAll(cfg.Repo.TempDir, 0755)

	storageConfig := configapp.StorageConfig{
		DataDir: cfg.Repo.DataDir,
		TempDir: cfg.Repo.TempDir,
		Aggregator: configapp.AggregatorStorageConfig{
			DatastoreDir: filepath.Join(cfg.Repo.DataDir, "aggregator", "datastore"),
		},
		Blobs: configapp.BlobStorageConfig{
			StoreDir: filepath.Join(cfg.Repo.DataDir, "blobs"),
			TempDir:  filepath.Join(cfg.Repo.TempDir, "storage"),
		},
		Claims: configapp.ClaimStorageConfig{
			StoreDir: filepath.Join(cfg.Repo.DataDir, "claim"),
		},
		Publisher: configapp.PublisherStorageConfig{
			StoreDir: filepath.Join(cfg.Repo.DataDir, "publisher"),
		},
		Receipts: configapp.ReceiptStorageConfig{
			StoreDir: filepath.Join(cfg.Repo.DataDir, "receipt"),
		},
		Allocations: configapp.AllocationStorageConfig{
			StoreDir: filepath.Join(cfg.Repo.DataDir, "allocation"),
		},
		Replicator: configapp.ReplicatorStorageConfig{
			DBPath: "", // Will be set below if needed
		},
	}

	// Set replicator DB path if PDP is configured or if we have a data dir
	if cfg.Repo.DataDir != "" {
		storageConfig.Replicator.DBPath = filepath.Join(cfg.Repo.DataDir, "aggregator", "jobqueue", "jobqueue.db")
	}

	return storageConfig
}

func buildExternalServicesConfig(cfg config.UCANServer) (configapp.ExternalServicesConfig, error) {
	var externalConfig configapp.ExternalServicesConfig

	// Upload service
	uploadDID, err := did.Parse(cfg.UploadServiceDID)
	if err != nil {
		return externalConfig, fmt.Errorf("parsing upload service DID: %w", err)
	}
	uploadURL, err := url.Parse(cfg.UploadServiceURL)
	if err != nil {
		return externalConfig, fmt.Errorf("parsing upload service URL: %w", err)
	}
	uploadChannel := ucanhttp.NewHTTPChannel(uploadURL)
	uploadConn, err := client.NewConnection(uploadDID, uploadChannel)
	if err != nil {
		return externalConfig, fmt.Errorf("creating upload service connection: %w", err)
	}
	externalConfig.UploadService = configapp.ServiceConnectionConfig{
		Connection: uploadConn,
	}

	// Indexing service
	indexingDID, err := did.Parse(cfg.IndexingServiceDID)
	if err != nil {
		return externalConfig, fmt.Errorf("parsing indexing service DID: %w", err)
	}
	indexingURL, err := url.Parse(cfg.IndexingServiceURL)
	if err != nil {
		return externalConfig, fmt.Errorf("parsing indexing service URL: %w", err)
	}
	indexingChannel := ucanhttp.NewHTTPChannel(indexingURL)
	indexingConn, err := client.NewConnection(indexingDID, indexingChannel)
	if err != nil {
		return externalConfig, fmt.Errorf("creating indexing service connection: %w", err)
	}

	// Parse indexing service proofs if provided
	var indexingProofs delegation.Proofs
	if cfg.IndexingServiceProof != "" {
		dlg, err := delegation.Parse(cfg.IndexingServiceProof)
		if err != nil {
			return externalConfig, fmt.Errorf("parsing indexing service proof: %w", err)
		}
		indexingProofs = delegation.Proofs{delegation.FromDelegation(dlg)}
	}

	externalConfig.IndexingService = configapp.IndexingServiceConfig{
		Connection: indexingConn,
		Proofs:     indexingProofs,
	}

	// PDP server (optional)
	if cfg.PDPServerURL != "" {
		pdpURL, err := url.Parse(cfg.PDPServerURL)
		if err != nil {
			return externalConfig, fmt.Errorf("parsing PDP server URL: %w", err)
		}
		externalConfig.PDPServer = &configapp.PDPServerConfig{
			URL:      pdpURL,
			ProofSet: cfg.ProofSet,
		}
	}

	return externalConfig, nil
}

func buildServicesConfig(cfg config.UCANServer, publicURL *url.URL) (configapp.ServicesConfig, error) {
	// Build publisher config
	pubMaddr, err := maurl.FromURL(publicURL)
	if err != nil {
		return configapp.ServicesConfig{}, fmt.Errorf("converting public URL to multiaddr: %w", err)
	}

	// Parse IPNI announce URLs
	var announceURLs []url.URL
	for _, s := range cfg.IPNIAnnounceURLs {
		u, err := url.Parse(s)
		if err != nil {
			return configapp.ServicesConfig{}, fmt.Errorf("parsing IPNI announce URL %s: %w", s, err)
		}
		announceURLs = append(announceURLs, *u)
	}

	// Handle blob address for PDP
	var blobAddr multiaddr.Multiaddr
	if cfg.PDPServerURL != "" {
		pdpURL, err := url.Parse(cfg.PDPServerURL)
		if err != nil {
			return configapp.ServicesConfig{}, fmt.Errorf("parsing PDP server URL: %w", err)
		}
		curioAddr, err := maurl.FromURL(pdpURL)
		if err != nil {
			return configapp.ServicesConfig{}, fmt.Errorf("converting PDP URL to multiaddr: %w", err)
		}
		blobAddr, err = mahttp.JoinPath(curioAddr, "piece/{blobCID}")
		if err != nil {
			return configapp.ServicesConfig{}, fmt.Errorf("joining blob path to PDP multiaddr: %w", err)
		}
	}

	return configapp.ServicesConfig{
		Publisher: configapp.PublisherConfig{
			PublicMaddr:   pubMaddr,
			AnnounceMaddr: pubMaddr,
			BlobMaddr:     blobAddr,
			AnnounceURLs:  announceURLs,
		},
		ServicePrincipalMapping: cfg.ServicePrincipalMapping,
	}, nil
}
