package serve

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/storacha/piri/cmd/cli/setup"
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
		"network",
		presets.DefaultNetwork.String(),
		"Network the node will operate on. This will set default values for service URLs and DIDs and contract addresses.",
	)
	cobra.CheckErr(viper.BindPFlag("network", FullCmd.Flags().Lookup("network")))

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
		presets.Services.IndexingServiceDID.String(),
		"DID of the indexing service",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("indexing-service-did"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.indexer.did", FullCmd.Flags().Lookup("indexing-service-did")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.indexer.did", "PIRI_INDEXING_SERVICE_DID"))

	FullCmd.Flags().String(
		"indexing-service-url",
		presets.Services.IndexingServiceURL.String(),
		"URL of the indexing service",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("indexing-service-url"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.indexer.url", FullCmd.Flags().Lookup("indexing-service-url")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.indexer.url", "PIRI_INDEXING_SERVICE_URL"))

	FullCmd.Flags().String(
		"egress-tracker-service-proof",
		"",
		"A delegation that allows the node to track egress with the egress tracker service",
	)
	cobra.CheckErr(viper.BindPFlag("ucan.services.etracker.proof", FullCmd.Flags().Lookup("egress-tracker-service-proof")))

	FullCmd.Flags().String(
		"egress-tracker-service-did",
		presets.Services.EgressTrackerServiceDID.String(),
		"DID of the egress tracker service",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("egress-tracker-service-did"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.etracker.did", FullCmd.Flags().Lookup("egress-tracker-service-did")))

	FullCmd.Flags().String(
		"egress-tracker-service-url",
		presets.Services.EgressTrackerServiceURL.String(),
		"URL of the egress tracker service",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("egress-tracker-service-url"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.etracker.url", FullCmd.Flags().Lookup("egress-tracker-service-url")))

	FullCmd.Flags().String(
		"egress-tracker-service-receipts-endpoint",
		presets.Services.EgressTrackerServiceURL.JoinPath("/receipts").String(),
		"URL of the egress tracker service receipts endpoint",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("egress-tracker-service-receipts-endpoint"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.etracker.receipts_endpoint", FullCmd.Flags().Lookup("egress-tracker-service-receipts-endpoint")))

	FullCmd.Flags().Int64(
		"egress-tracker-service-max-batch-size-bytes",
		// default: 100MiB
		100*1024*1024,
		"Maximum batch size in bytes for egress tracker service. It should be between 10MiB and 1GiB",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("egress-tracker-service-max-batch-size-bytes"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.etracker.max_batch_size_bytes", FullCmd.Flags().Lookup("egress-tracker-service-max-batch-size-bytes")))

	FullCmd.Flags().String(
		"upload-service-did",
		presets.Services.UploadServiceDID.String(),
		"DID of the upload service",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("upload-service-did"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.upload.did", FullCmd.Flags().Lookup("upload-service-did")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.upload.did", "PIRI_UPLOAD_SERVICE_DID"))

	FullCmd.Flags().String(
		"upload-service-url",
		presets.Services.UploadServiceURL.String(),
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
			for _, u := range presets.Services.IPNIAnnounceURLs {
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
		presets.Services.PrincipalMapping,
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
		"verifier-address",
		presets.SmartContracts.Verifier.Hex(),
		"[Advanced] PDP Verifier contract address. Only change if you know what you're doing. Use --network flag to set proper defaults.",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("verifier-address"))
	cobra.CheckErr(viper.BindPFlag("pdp.contracts.verifier", FullCmd.Flags().Lookup("verifier-address")))

	FullCmd.Flags().String(
		"provider-registry-address",
		presets.SmartContracts.ProviderRegistry.Hex(),
		"[Advanced] Provider Registry contract address. Only change if you know what you're doing. Use --network flag to set proper defaults.",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("provider-registry-address"))
	cobra.CheckErr(viper.BindPFlag("pdp.contracts.provider_registry", FullCmd.Flags().Lookup("provider-registry-address")))

	FullCmd.Flags().String(
		"service-address",
		presets.SmartContracts.Service.Hex(),
		"[Advanced] PDP Service contract address. Only change if you know what you're doing. Use --network flag to set proper defaults.",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("service-address"))
	cobra.CheckErr(viper.BindPFlag("pdp.contracts.service", FullCmd.Flags().Lookup("service-address")))

	FullCmd.Flags().String(
		"service-view-address",
		presets.SmartContracts.ServiceView.Hex(),
		"[Advanced] Service View contract address. Only change if you know what you're doing. Use --network flag to set proper defaults.",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("service-view-address"))
	cobra.CheckErr(viper.BindPFlag("pdp.contracts.service_view", FullCmd.Flags().Lookup("service-view-address")))

	FullCmd.Flags().String(
		"chain-id",
		presets.SmartContracts.ChainID.String(),
		"[Advanced] Filecoin chain ID (314 for mainnet, 314159 for calibration). Only change if you know what you're doing. Use --network flag to set proper defaults.",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("chain-id"))
	cobra.CheckErr(viper.BindPFlag("pdp.chain_id", FullCmd.Flags().Lookup("chain-id")))

	FullCmd.Flags().String(
		"contract-address",
		"",
		"The ethereum address of the PDP Contract",
	)
	cobra.CheckErr(FullCmd.Flags().MarkDeprecated("contract-address", "The contract-address flag is deprecated. Use --verifier-address instead."))

	FullCmd.Flags().String(
		"contract-signing-service-endpoint",
		presets.Services.SigningServiceEndpoint.String(),
		"Endpoint of the contract signing service",
	)
	cobra.CheckErr(viper.BindPFlag("pdp.signing_service.endpoint", FullCmd.Flags().Lookup("contract-signing-service-endpoint")))
	cobra.CheckErr(FullCmd.Flags().MarkHidden("contract-signing-service-endpoint"))

}

func loadPresets(cmd *cobra.Command) error {
	networkStr := viper.GetString("network")
	network, err := presets.ParseNetwork(networkStr)
	if err != nil {
		return fmt.Errorf("invalid network %q: %w", networkStr, err)
	}
	preset := presets.GetPreset(network)

	// Update the global preset variables so they can be used by config loading
	presets.Services = preset.Services
	presets.SmartContracts = preset.SmartContracts

	// Apply service presets only if flags weren't explicitly set by user
	if !cmd.Flags().Changed("indexing-service-did") {
		viper.Set("ucan.services.indexer.did", preset.Services.IndexingServiceDID.String())
	}
	if !cmd.Flags().Changed("indexing-service-url") {
		viper.Set("ucan.services.indexer.url", preset.Services.IndexingServiceURL.String())
	}
	if !cmd.Flags().Changed("egress-tracker-service-did") {
		viper.Set("ucan.services.etracker.did", preset.Services.EgressTrackerServiceDID.String())
	}
	if !cmd.Flags().Changed("egress-tracker-service-url") {
		viper.Set("ucan.services.etracker.url", preset.Services.EgressTrackerServiceURL.String())
	}
	if !cmd.Flags().Changed("egress-tracker-service-receipts-endpoint") {
		viper.Set("ucan.services.etracker.receipts_endpoint", preset.Services.EgressTrackerServiceURL.JoinPath("/receipts").String())
	}
	if !cmd.Flags().Changed("upload-service-did") {
		viper.Set("ucan.services.upload.did", preset.Services.UploadServiceDID.String())
	}
	if !cmd.Flags().Changed("upload-service-url") {
		viper.Set("ucan.services.upload.url", preset.Services.UploadServiceURL.String())
	}
	if !cmd.Flags().Changed("ipni-announce-urls") {
		urls := make([]string, len(preset.Services.IPNIAnnounceURLs))
		for i, u := range preset.Services.IPNIAnnounceURLs {
			urls[i] = u.String()
		}
		viper.Set("ucan.services.publisher.ipni_announce_urls", urls)
	}
	if !cmd.Flags().Changed("service-principal-mapping") {
		viper.Set("ucan.services.principal_mapping", preset.Services.PrincipalMapping)
	}
	if !cmd.Flags().Changed("contract-signing-service-endpoint") && preset.Services.SigningServiceEndpoint != nil {
		viper.Set("pdp.signing_service.endpoint", preset.Services.SigningServiceEndpoint.String())
	}

	// Apply smart contract presets only if flags weren't explicitly set by user
	if !cmd.Flags().Changed("verifier-address") {
		viper.Set("pdp.contracts.verifier", preset.SmartContracts.Verifier.Hex())
	}
	if !cmd.Flags().Changed("provider-registry-address") {
		viper.Set("pdp.contracts.provider_registry", preset.SmartContracts.ProviderRegistry.Hex())
	}
	if !cmd.Flags().Changed("service-address") {
		viper.Set("pdp.contracts.service", preset.SmartContracts.Service.Hex())
	}
	if !cmd.Flags().Changed("service-view-address") {
		viper.Set("pdp.contracts.service_view", preset.SmartContracts.ServiceView.Hex())
	}
	if !cmd.Flags().Changed("chain-id") {
		viper.Set("pdp.chain_id", preset.SmartContracts.ChainID.String())
	}

	return nil
}

func fullServer(cmd *cobra.Command, _ []string) error {
	// Apply network presets before loading config, but only for flags that weren't explicitly set
	if err := loadPresets(cmd); err != nil {
		return fmt.Errorf("loading presets: %w", err)
	}

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

		fx.StopTimeout(setup.PiriServerShutdownTimeout),

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
					log.Infof("Shutting down piri...this may take up to %s", setup.PiriServerShutdownTimeout)
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
