package serve

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap/zapcore"

	"github.com/storacha/piri/cmd/cliutil"
	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/fx/app"
	"github.com/storacha/piri/pkg/presets"
)

var (
	FullCmd = &cobra.Command{
		Use:   "full",
		Short: "Start the full piri server!",
		Args:  cobra.NoArgs,
		RunE:  fullServer,
	}
)

func init() {
	FullCmd.Flags().String(
		"pdp-server-url",
		"",
		"URL used to connect to pdp server",
	)
	cobra.CheckErr(viper.BindPFlag("pdp_server_url", FullCmd.Flags().Lookup("pdp-server-url")))

	FullCmd.Flags().Uint64(
		"proof-set",
		0,
		"Proofset to use with PDP",
	)
	cobra.CheckErr(viper.BindPFlag("ucan.proof_set", FullCmd.Flags().Lookup("proof-set")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.proof_set", "PIRI_PROOF_SET"))

	FullCmd.Flags().String(
		"indexing-service-proof",
		"",
		"A delegation that allows the node to cache claims with the indexing service",
	)
	cobra.CheckErr(viper.BindPFlag("ucan.services.indexer.proof", FullCmd.Flags().Lookup("indexing-service-proof")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.indexer.proof", "PIRI_INDEXING_SERVICE_PROOF"))

	FullCmd.Flags().String(
		"indexing-service-did",
		presets.IndexingServiceDID.String(),
		"DID of the indexing service",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("indexing-service-did"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.indexer.did", FullCmd.Flags().Lookup("indexing-service-did")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.indexer.did", "PIRI_INDEXING_SERVICE_DID"))

	FullCmd.Flags().String(
		"indexing-service-url",
		presets.IndexingServiceURL.String(),
		"URL of the indexing service",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("indexing-service-url"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.indexer.url", FullCmd.Flags().Lookup("indexing-service-url")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.indexer.url", "PIRI_INDEXING_SERVICE_URL"))

	FullCmd.Flags().String(
		"upload-service-did",
		presets.UploadServiceDID.String(),
		"DID of the upload service",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("upload-service-did"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.upload.did", FullCmd.Flags().Lookup("upload-service-did")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.upload.did", "PIRI_UPLOAD_SERVICE_DID"))

	FullCmd.Flags().String(
		"upload-service-url",
		presets.UploadServiceURL.String(),
		"URL of the upload service",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("upload-service-url"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.upload.url", FullCmd.Flags().Lookup("upload-service-url")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.upload.did", "PIRI_UPLOAD_SERVICE_URL"))

	FullCmd.Flags().StringSlice(
		"ipni-announce-urls",
		func() []string {
			out := make([]string, 0)
			for _, u := range presets.IPNIAnnounceURLs {
				out = append(out, u.String())
			}
			return out
		}(),
		"A list of IPNI announce URLs")
	cobra.CheckErr(FullCmd.Flags().MarkHidden("ipni-announce-urls"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.publisher.ipni_announce_urls", FullCmd.Flags().Lookup("ipni-announce-urls")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.publisher.ipni_announce_urls", "PIRI_IPNI_ANNOUNCE_URLS"))

	FullCmd.Flags().StringToString(
		"service-principal-mapping",
		presets.PrincipalMapping,
		"Mapping of service DIDs to principal DIDs",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("service-principal-mapping"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.principal_mapping", FullCmd.Flags().Lookup("service-principal-mapping")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.principal_mapping", "PIRI_SERVICE_PRINCIPAL_MAPPING"))

	FullCmd.Flags().String(
		"lotus-url",
		"",
		"A websocket url for lotus node",
	)
	cobra.CheckErr(viper.BindPFlag("pdp.lotus_endpoint", FullCmd.Flags().Lookup("lotus-url")))

	FullCmd.Flags().String(
		"owner-address",
		"",
		"The ethereum address to submit PDP Proofs with (must be in piri wallet - see `piri wallet` command for help)",
	)
	cobra.CheckErr(viper.BindPFlag("pdp.owner_address", FullCmd.Flags().Lookup("owner-address")))

	FullCmd.Flags().String(
		"contract-address",
		"0x6170dE2b09b404776197485F3dc6c968Ef948505", // NB(forrest): default to calibration contract addrese
		"The ethereum address of the PDP Contract",
	)
	cobra.CheckErr(viper.BindPFlag("pdp.contract_address", FullCmd.Flags().Lookup("contract-address")))

}

func fullServer(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	userCfg, err := config.Load[config.FullServerConfig]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	appCfg, err := userCfg.ToAppConfig()
	if err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	fxApp := fx.New(
		// if a panic occurs during operation, recover from it and exit (somewhat) gracefully.
		fx.RecoverFromPanics(),
		// provide fx with our logger for its events logged at debug level.
		// any fx errors will still be logged at the error level.
		fx.WithLogger(func() fxevent.Logger {
			el := &fxevent.ZapLogger{Logger: log.Desugar()}
			el.UseLogLevel(zapcore.DebugLevel)
			return el
		}),

		// common dependencies of the PDP and UCAN module:
		//   - identity
		//   - http server
		//   - databases & datastores
		app.CommonModules(appCfg),

		// ucan service dependencies:
		//  - http handlers
		//    - ucan specific handlers, blob allocate and accept, replicate, etc.
		//  - blob, claim, publisher, replicator, and storage services
		app.UCANModule,

		// pdp service dependencies:
		//  - lotus, eth, and contract clients
		//  - piece aggregator
		//  - task and chain scheduler w/ their related tasks
		//  - http handlers
		//    - create proof set, add root, upload piece, etc.
		//  - address wallet
		app.PDPModule,
	)

	// ensure the application was initialized correctly
	if err := fxApp.Err(); err != nil {
		return fmt.Errorf("initalizing piri: %w", err)
	}

	// start the application, triggering lifecycle hooks to start various services and systems
	if err := fxApp.Start(ctx); err != nil {
		return fmt.Errorf("starting piri: %w", err)
	}

	go func() {
		// sleep a bit allowing for initial logs to write before printing hello
		time.Sleep(time.Second)
		cliutil.PrintHero(cmd.OutOrStdout(), appCfg.Identity.Signer.DID())
		cmd.Println("Piri Running on: " + appCfg.Server.Host + ":" + strconv.Itoa(int(appCfg.Server.Port)))
		cmd.Println("Piri Public Endpoint: " + appCfg.Server.PublicURL.String())
	}()

	// block: wait for the application to receive a shutdown signal
	<-ctx.Done()
	log.Info("received shutdown signal, beginning graceful shutdown")

	shutdownTimeout := 5 * time.Second
	// Stop the application, with a `shutdownTimeout grace period.
	stopCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	log.Info("stopping piri...")
	if err := fxApp.Stop(stopCtx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			log.Errorf("graceful shutdown timed out after %s", shutdownTimeout.String())
		}
		return fmt.Errorf("stopping piri: %w", err)
	}
	log.Info("piri stopped successfully")

	// flush any logs before exiting.
	_ = log.Sync()
	return nil

}
