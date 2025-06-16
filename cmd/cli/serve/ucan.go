package serve

import (
	"context"
	"fmt"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/fx"

	"github.com/storacha/piri/cmd/cliutil"
	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/datastores"
	"github.com/storacha/piri/pkg/server"
	"github.com/storacha/piri/pkg/services"
	servicesconfig "github.com/storacha/piri/pkg/services/config"
)

var (
	UCANCmd = &cobra.Command{
		Use:   "ucan",
		Short: "Start the UCAN server.",
		Args:  cobra.NoArgs,
		RunE:  startServer,
	}
)

func init() {
	UCANCmd.Flags().String(
		"host",
		config.DefaultUCANServer.Host,
		"Host to listen on")
	cobra.CheckErr(viper.BindPFlag("host", UCANCmd.Flags().Lookup("host")))

	UCANCmd.Flags().Uint(
		"port",
		config.DefaultUCANServer.Port,
		"Port to listen on",
	)
	cobra.CheckErr(viper.BindPFlag("port", UCANCmd.Flags().Lookup("port")))

	UCANCmd.Flags().String(
		"public-url",
		config.DefaultUCANServer.PublicURL,
		"URL the node is publicly accessible at and exposed to other storacha services",
	)
	cobra.CheckErr(viper.BindPFlag("public_url", UCANCmd.Flags().Lookup("public-url")))

	UCANCmd.Flags().String(
		"pdp-server-url",
		config.DefaultUCANServer.PDPServerURL,
		"URL used to connect to pdp server",
	)
	cobra.CheckErr(viper.BindPFlag("pdp_server_url", UCANCmd.Flags().Lookup("pdp-server-url")))

	UCANCmd.Flags().Uint64(
		"proof-set",
		config.DefaultUCANServer.ProofSet,
		"Proofset to use with PDP",
	)
	cobra.CheckErr(viper.BindPFlag("proof_set", UCANCmd.Flags().Lookup("proof-set")))
	UCANCmd.MarkFlagsRequiredTogether("pdp-server-url", "proof-set")

	UCANCmd.Flags().String(
		"indexing-service-proof",
		config.DefaultUCANServer.IndexingServiceProof,
		"A delegation that allows the node to cache claims with the indexing service",
	)
	cobra.CheckErr(viper.BindPFlag("indexing_service_proof", UCANCmd.Flags().Lookup("indexing-service-proof")))

	UCANCmd.Flags().String(
		"indexing-service-did",
		config.DefaultUCANServer.IndexingServiceDID,
		"DID of the indexing service",
	)
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("indexing-service-did"))
	cobra.CheckErr(viper.BindPFlag("indexing_service_did", UCANCmd.Flags().Lookup("indexing-service-did")))

	UCANCmd.Flags().String(
		"indexing-service-url",
		config.DefaultUCANServer.IndexingServiceURL,
		"URL of the indexing service",
	)
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("indexing-service-url"))
	cobra.CheckErr(viper.BindPFlag("indexing_service_url", UCANCmd.Flags().Lookup("indexing-service-url")))

	UCANCmd.Flags().String(
		"upload-service-did",
		config.DefaultUCANServer.UploadServiceDID,
		"DID of the upload service",
	)
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("upload-service-did"))
	cobra.CheckErr(viper.BindPFlag("upload_service_did", UCANCmd.Flags().Lookup("upload-service-did")))

	UCANCmd.Flags().String(
		"upload-service-url",
		config.DefaultUCANServer.UploadServiceURL,
		"URL of the upload service",
	)
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("upload-service-url"))
	cobra.CheckErr(viper.BindPFlag("upload_service_url", UCANCmd.Flags().Lookup("upload-service-url")))

	UCANCmd.Flags().StringSlice(
		"ipni-announce-urls",
		config.DefaultUCANServer.IPNIAnnounceURLs,
		"A list of IPNI announce URLs")
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("ipni-announce-urls"))
	cobra.CheckErr(viper.BindPFlag("ipni_announce_urls", UCANCmd.Flags().Lookup("ipni-announce-urls")))

	UCANCmd.Flags().StringToString(
		"service-principal-mapping",
		config.DefaultUCANServer.ServicePrincipalMapping,
		"Mapping of service DIDs to principal DIDs",
	)
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("service-principal-mapping"))
	cobra.CheckErr(viper.BindPFlag("service_principal_mapping", UCANCmd.Flags().Lookup("service-principal-mapping")))

}

func startServer(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load[config.UCANServer]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Transform user configuration to storage configuration
	storageCfg, err := app.TransformUCANConfig(cfg)
	if err != nil {
		return fmt.Errorf("transforming config: %w", err)
	}

	// Create fx app with all dependencies
	app := fx.New(
		// Supply the pre-transformed storage configuration
		fx.Supply(storageCfg),

		// Include filesystem datastores
		datastores.FilesystemModule,

		// Include service configuration
		servicesconfig.Module,

		// Include all services
		services.ServiceModule,
		// Include HTTP handlers
		services.HTTPHandlersModule,
		// Include UCAN Service Methods
		services.UCANMethodsModule,

		// Include Echo server module
		server.Module,

		// Start the HTTP server
		fx.Invoke(func(lc fx.Lifecycle, e *echo.Echo, strgCfg app.Config) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					go func() {
						addr := fmt.Sprintf(":%d", cfg.Port)
						log.Infof("Starting server on %s", addr)
						if err := e.Start(addr); err != nil {
							log.Errorf("Server error: %v", err)
						}
					}()

					// Print hero banner after a short delay
					go func() {
						time.Sleep(time.Millisecond * 50)
						cliutil.PrintHero(strgCfg.ID.DID())
					}()

					return nil
				},
				OnStop: func(ctx context.Context) error {
					log.Info("Stopping server")
					return e.Shutdown(ctx)
				},
			})
		}),
	)

	// Start the app
	if err := app.Start(ctx); err != nil {
		return fmt.Errorf("starting fx app: %w", err)
	}

	// Wait for shutdown signal
	<-app.Done()

	// Stop the app
	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.Stop(stopCtx); err != nil {
		return fmt.Errorf("stopping fx app: %w", err)
	}

	return nil

}
