package serve

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/storacha/go-ucanto/principal"
	"go.uber.org/fx"

	"github.com/storacha/piri/cmd/cliutil"
	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/fx/app"
)

var (
	FullCmd = &cobra.Command{
		Use:   "full",
		Short: "Start the full server.",
		Args:  cobra.NoArgs,
		RunE:  startFullServer,
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

	// Create the fx application with all modules
	fxApp := fx.New(
		// supply the user provided configuration
		fx.Supply(cfg),
		app.FullModule,
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
