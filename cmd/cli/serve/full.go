package serve

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap/zapcore"

	"github.com/storacha/piri/cmd/cliutil"
	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/fx/app"
	"github.com/storacha/piri/pkg/presets"
	"github.com/storacha/piri/pkg/telemetry"
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
		presets.PDPRecordKeeperAddress,
		"The ethereum address of the PDP Contract",
	)
	cobra.CheckErr(viper.BindPFlag("pdp.contract_address", FullCmd.Flags().Lookup("contract-address")))

}

func fullServer(cmd *cobra.Command, _ []string) error {
	userCfg, err := config.Load[config.FullServerConfig]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	appCfg, err := userCfg.ToAppConfig()
	if err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	// build our beloved Piri node
	piri := fx.New(
		// if a panic occurs during operation, recover from it and exit (somewhat) gracefully.
		fx.RecoverFromPanics(),

		// provide fx with our logger for its events logged at debug level.
		// any fx errors will still be logged at the error level.
		fx.WithLogger(func() fxevent.Logger {
			el := &fxevent.ZapLogger{Logger: log.Desugar()}
			el.UseLogLevel(zapcore.DebugLevel)
			return el
		}),

		fx.StopTimeout(cliutil.PiriServerShutdownTimeout),

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

		// Post-startup operations: print server info and record telemetry
		fx.Invoke(func(lc fx.Lifecycle) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					// Print server startup information
					cliutil.PrintHero(cmd.OutOrStdout(), appCfg.Identity.Signer.DID())
					cmd.Println("Piri Running on: " + appCfg.Server.Host + ":" + strconv.Itoa(int(appCfg.Server.Port)))
					cmd.Println("Piri Public Endpoint: " + appCfg.Server.PublicURL.String())

					// Record server telemetry
					telemetry.RecordServerInfo(ctx, "full",
						telemetry.StringAttr("did", appCfg.Identity.Signer.DID().String()),
						telemetry.StringAttr("owner_address", appCfg.PDPService.OwnerAddress.String()),
						telemetry.StringAttr("public_url", appCfg.Server.PublicURL.String()),
						telemetry.Int64Attr("proof_set", int64(appCfg.UCANService.ProofSetID)),
					)
					return nil
				},
				OnStop: func(ctx context.Context) error {
					log.Infof("Shutting down piri...this may take up to %s", cliutil.PiriServerShutdownTimeout)
					return nil
				},
			})
		}),
	)

	// valid the app was built successfully, an error here means a missing dep, i.e. a developer error (we never write errors...)
	if err := piri.Err(); err != nil {
		return fmt.Errorf("building piri: %w", err)
	}

	// run the app, when an interrupt signal is sent to the process, this method ends.
	// any errors encountered during shutdown will be exposed via logs
	piri.Run()

	return nil
}
