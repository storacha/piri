package serve

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/fx/app"
	"github.com/storacha/piri/pkg/presets"
)

var FullCmd = &cobra.Command{
	Use:   "full",
	Args:  cobra.NoArgs,
	Short: `Start a Full server`,
	RunE:  doServe,
}

func init() {
	FullCmd.Flags().String(
		"host",
		"localhost",
		"Host to listen on")
	cobra.CheckErr(viper.BindPFlag("server.host", FullCmd.Flags().Lookup("host")))

	FullCmd.Flags().Uint(
		"port",
		3000,
		"Port to listen on",
	)
	cobra.CheckErr(viper.BindPFlag("server.port", FullCmd.Flags().Lookup("port")))

	FullCmd.Flags().String(
		"public-url",
		"http://localhost:3000",
		"URL the node is publicly accessible at and exposed to other storacha services",
	)
	cobra.CheckErr(viper.BindPFlag("server.public_url", FullCmd.Flags().Lookup("public-url")))

	FullCmd.Flags().String(
		"indexing-service-did",
		presets.IndexingServiceDID.String(),
		"DID of the indexing service",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("indexing-service-did"))
	cobra.CheckErr(viper.BindPFlag("services.indexer.did", FullCmd.Flags().Lookup("indexing-service-did")))

	FullCmd.Flags().String(
		"indexing-service-url",
		presets.IndexingServiceURL.String(),
		"URL of the indexing service",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("indexing-service-url"))
	cobra.CheckErr(viper.BindPFlag("services.indexer.url", FullCmd.Flags().Lookup("indexing-service-url")))

	FullCmd.Flags().String(
		"upload-service-did",
		presets.UploadServiceDID.String(),
		"DID of the upload service",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("upload-service-did"))
	cobra.CheckErr(viper.BindPFlag("services.upload.did", FullCmd.Flags().Lookup("upload-service-did")))

	FullCmd.Flags().String(
		"upload-service-url",
		presets.UploadServiceURL.String(),
		"URL of the upload service",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("upload-service-url"))
	cobra.CheckErr(viper.BindPFlag("services.upload.url", FullCmd.Flags().Lookup("upload-service-url")))

	FullCmd.Flags().StringSlice(
		"ipni-announce-urls",
		func() []string {
			out := make([]string, len(presets.IPNIAnnounceURLs))
			for i, p := range presets.IPNIAnnounceURLs {
				out[i] = p.String()
			}
			return out
		}(),
		"A list of IPNI announce URLs")
	cobra.CheckErr(FullCmd.Flags().MarkHidden("ipni-announce-urls"))
	cobra.CheckErr(viper.BindPFlag("services.publisher.ipni_announce_urls", FullCmd.Flags().Lookup("ipni-announce-urls")))

	FullCmd.Flags().StringToString(
		"service-principal-mapping",
		presets.PrincipalMapping,
		"Mapping of service DIDs to principal DIDs",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("service-principal-mapping"))
	cobra.CheckErr(viper.BindPFlag("services.principal_mapping", FullCmd.Flags().Lookup("service-principal-mapping")))

	FullCmd.Flags().String(
		"owner-address",
		"",
		"Ethereum style owner address (your address)")
	cobra.CheckErr(viper.BindPFlag("pdp.owner_address", FullCmd.Flags().Lookup("owner-address")))

	FullCmd.Flags().String(
		"contract-address",
		"0x6170dE2b09b404776197485F3dc6c968Ef948505", // NB: this is the calibration network address
		"Ethereum style address of PDP Service contract")
	cobra.CheckErr(viper.BindPFlag("pdp.contract_address", FullCmd.Flags().Lookup("contract-address")))

	FullCmd.Flags().String(
		"lotus-endpoint",
		"",
		"API endpoint of a lotus node")
	cobra.CheckErr(viper.BindPFlag("pdp.lotus_endpoint", FullCmd.Flags().Lookup("lotus-endpoint")))

	FullCmd.Flags().Uint64(
		"proof-set-id",
		0,
		"The Proof Set ID to operate with")
	cobra.CheckErr(viper.BindPFlag("pdp.proof_set_id", FullCmd.Flags().Lookup("proof-set-id")))
}

func doServe(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	// load and validate the PDPServer configuration, applying all flags, env vars, and config file to config.
	// Failing if a required field is not present
	userCfg, err := config.Load[config.Full]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	appCfg, err := userCfg.ToAppConfig()
	if err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	fxApp := fx.New(
		// configure how fx behaves
		fx.RecoverFromPanics(),
		fx.NopLogger,

		// common dependencies of the PDP and UCAN module
		app.CommonModule(appCfg),

		// PDP Module provides PDP related bits
		app.PDPModule,

		// UCAN related bits
		app.UCANModule,
	)

	if err := fxApp.Err(); err != nil {
		viz, vizErr := fx.VisualizeError(err)
		if vizErr == nil {
			cmd.Println(viz)
		}
		return fmt.Errorf("initalizing piri: %w", err)
	}

	if err := fxApp.Start(ctx); err != nil {
		return fmt.Errorf("starting piri: %w", err)
	}

	signal := <-fxApp.Done()
	log.Infow("receieved shutdown", "signal", signal)

	// Stop the application, with a 15-second grace period
	stopCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	log.Info("stopping piri")
	if err := fxApp.Stop(stopCtx); err != nil {
		return fmt.Errorf("stopping piri: %w", err)
	}

	return nil
}
