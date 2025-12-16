package serve

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/storacha/go-ucanto/did"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap/zapcore"

	"github.com/storacha/piri/cmd/cli/setup"
	"github.com/storacha/piri/cmd/cliutil"
	"github.com/storacha/piri/pkg/config"
	appconfig "github.com/storacha/piri/pkg/config/app"
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
		"",
		fmt.Sprintf("Network the node will operate on. This will set default values for service URLs and DIDs and contract addresses. Available values are: %q", presets.AvailableNetworks),
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("network"))
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
		"",
		"[Advanced] DID of the indexing service. Only change if you know what you're doing. Use --network flag to set proper defaults.",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("indexing-service-did"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.indexer.did", FullCmd.Flags().Lookup("indexing-service-did")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.indexer.did", "PIRI_INDEXING_SERVICE_DID"))

	FullCmd.Flags().String(
		"indexing-service-url",
		"",
		"[Advanced] URL of the indexing service. Only change if you know what you're doing. Use --network flag to set proper defaults.",
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
		"",
		"[Advanced] DID of the egress tracker service. Only change if you know what you're doing. Use --network flag to set proper defaults.",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("egress-tracker-service-did"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.etracker.did", FullCmd.Flags().Lookup("egress-tracker-service-did")))

	FullCmd.Flags().String(
		"egress-tracker-service-url",
		"",
		"[Advanced] URL of the egress tracker service. Only change if you know what you're doing. Use --network flag to set proper defaults.",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("egress-tracker-service-url"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.etracker.url", FullCmd.Flags().Lookup("egress-tracker-service-url")))

	FullCmd.Flags().String(
		"egress-tracker-service-receipts-endpoint",
		"",
		"[Advanced] URL of the egress tracker service receipts endpoint. Only change if you know what you're doing. Use --network flag to set proper defaults.",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("egress-tracker-service-receipts-endpoint"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.etracker.receipts_endpoint", FullCmd.Flags().Lookup("egress-tracker-service-receipts-endpoint")))

	FullCmd.Flags().Int64(
		"egress-tracker-service-max-batch-size-bytes",
		config.DefaultMinimumEgressBatchSize,
		"Maximum batch size in bytes for egress tracker service. It should be between 10MiB and 1GiB",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("egress-tracker-service-max-batch-size-bytes"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.etracker.max_batch_size_bytes", FullCmd.Flags().Lookup("egress-tracker-service-max-batch-size-bytes")))

	FullCmd.Flags().String(
		"upload-service-did",
		"",
		"[Advanced] DID of the upload service. Only change if you know what you're doing. Use --network flag to set proper defaults.",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("upload-service-did"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.upload.did", FullCmd.Flags().Lookup("upload-service-did")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.upload.did", "PIRI_UPLOAD_SERVICE_DID"))

	FullCmd.Flags().String(
		"upload-service-url",
		"",
		"[Advanced] URL of the upload service. Only change if you know what you're doing. Use --network flag to set proper defaults.",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("upload-service-url"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.upload.url", FullCmd.Flags().Lookup("upload-service-url")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.upload.did", "PIRI_UPLOAD_SERVICE_URL"))

	FullCmd.Flags().StringSlice(
		"ipni-announce-urls",
		[]string{},
		"[Advanced] A list of IPNI announce URLs. Only change if you know what you're doing. Use --network flag to set proper defaults.",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("ipni-announce-urls"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.publisher.ipni_announce_urls", FullCmd.Flags().Lookup("ipni-announce-urls")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.publisher.ipni_announce_urls", "PIRI_IPNI_ANNOUNCE_URLS"))

	FullCmd.Flags().StringToString(
		"service-principal-mapping",
		map[string]string{},
		"[Advanced] Mapping of service DIDs to principal DIDs. Only change if you know what you're doing. Use --network flag to set proper defaults.",
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
		"",
		"[Advanced] PDP Verifier contract address. Only change if you know what you're doing. Use --network flag to set proper defaults.",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("verifier-address"))
	cobra.CheckErr(viper.BindPFlag("pdp.contracts.verifier", FullCmd.Flags().Lookup("verifier-address")))

	FullCmd.Flags().String(
		"provider-registry-address",
		"",
		"[Advanced] Provider Registry contract address. Only change if you know what you're doing. Use --network flag to set proper defaults.",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("provider-registry-address"))
	cobra.CheckErr(viper.BindPFlag("pdp.contracts.provider_registry", FullCmd.Flags().Lookup("provider-registry-address")))

	FullCmd.Flags().String(
		"service-address",
		"",
		"[Advanced] PDP Service contract address. Only change if you know what you're doing. Use --network flag to set proper defaults.",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("service-address"))
	cobra.CheckErr(viper.BindPFlag("pdp.contracts.service", FullCmd.Flags().Lookup("service-address")))

	FullCmd.Flags().String(
		"service-view-address",
		"",
		"[Advanced] Service View contract address. Only change if you know what you're doing. Use --network flag to set proper defaults.",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("service-view-address"))
	cobra.CheckErr(viper.BindPFlag("pdp.contracts.service_view", FullCmd.Flags().Lookup("service-view-address")))

	FullCmd.Flags().String(
		"chain-id",
		"",
		"[Advanced] Filecoin chain ID (314 for mainnet, 314159 for calibration). Only change if you know what you're doing. Use --network flag to set proper defaults.",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("chain-id"))
	cobra.CheckErr(viper.BindPFlag("pdp.chain_id", FullCmd.Flags().Lookup("chain-id")))

	FullCmd.Flags().String(
		"payer-address",
		"",
		"[Advanced] Address of the wallet that pays SPs. Only change if you know what you're doing. Use --network flag to set proper defaults.",
	)
	cobra.CheckErr(FullCmd.Flags().MarkHidden("payer-address"))
	cobra.CheckErr(viper.BindPFlag("pdp.payer_address", FullCmd.Flags().Lookup("payer-address")))

	FullCmd.Flags().String(
		"contract-address",
		"",
		"The ethereum address of the PDP Contract",
	)
	cobra.CheckErr(FullCmd.Flags().MarkDeprecated("contract-address", "The contract-address flag is deprecated. Use --verifier-address instead."))

	FullCmd.Flags().String(
		"contract-signing-service-did",
		"",
		"[Advanced] DID of the contract signing service. Only change if you know what you're doing. Use --network flag to set proper defaults.",
	)
	cobra.CheckErr(viper.BindPFlag("pdp.signing_service.did", FullCmd.Flags().Lookup("contract-signing-service-did")))
	cobra.CheckErr(FullCmd.Flags().MarkHidden("contract-signing-service-did"))

	FullCmd.Flags().String(
		"contract-signing-service-url",
		"",
		"[Advanced] URL of the contract signing service. Only change if you know what you're doing. Use --network flag to set proper defaults.",
	)
	cobra.CheckErr(viper.BindPFlag("pdp.signing_service.url", FullCmd.Flags().Lookup("contract-signing-service-url")))
	cobra.CheckErr(FullCmd.Flags().MarkHidden("contract-signing-service-url"))
}

func loadPresets() error {
	networkStr := viper.GetString("network")
	network, err := presets.ParseNetwork(networkStr)
	if err != nil {
		return err
	}

	preset, err := presets.GetPreset(network)
	if err != nil {
		return err
	}

	// given the network, set the _default_ configuration values. These values will apply iff other config: flag, envvar,
	// file are not provided. This allows users to selectively apply the changes they want via config sources, while
	// using the remaining defaults for the provided network
	urls := make([]string, len(preset.Services.IPNIAnnounceURLs))
	for i, u := range preset.Services.IPNIAnnounceURLs {
		urls[i] = u.String()
	}
	viper.SetDefault("ucan.services.publisher.ipni_announce_urls", urls)
	viper.SetDefault("ucan.services.principal_mapping", preset.Services.PrincipalMapping)

	viper.SetDefault("ucan.services.indexer.url", preset.Services.IndexingServiceURL.String())
	viper.SetDefault("ucan.services.indexer.did", preset.Services.IndexingServiceDID.String())
	viper.SetDefault("ucan.services.etracker.url", preset.Services.EgressTrackerServiceURL.String())
	viper.SetDefault("ucan.services.etracker.did", preset.Services.EgressTrackerServiceDID.String())
	viper.SetDefault("ucan.services.etracker.receipts_endpoint", preset.Services.EgressTrackerServiceURL.JoinPath("/receipts").String())
	viper.SetDefault("ucan.services.upload.url", preset.Services.UploadServiceURL.String())
	viper.SetDefault("ucan.services.upload.did", preset.Services.UploadServiceDID.String())

	// the registrar and the signing service are not present in all environments
	if preset.Services.SigningServiceURL != nil {
		viper.SetDefault("pdp.signing_service.url", preset.Services.SigningServiceURL.String())
	}
	if preset.Services.SigningServiceDID != did.Undef {
		viper.SetDefault("pdp.signing_service.did", preset.Services.SigningServiceDID.String())
	}
	if preset.Services.RegistrarServiceURL != nil {
		viper.SetDefault("pdp.registrar_service.url", preset.Services.RegistrarServiceURL.String())
	}

	// smart contract defaults
	viper.SetDefault("pdp.contracts.verifier", preset.SmartContracts.Verifier.String())
	viper.SetDefault("pdp.contracts.provider_registry", preset.SmartContracts.ProviderRegistry.String())
	viper.SetDefault("pdp.contracts.service", preset.SmartContracts.Service.String())
	viper.SetDefault("pdp.contracts.service_view", preset.SmartContracts.ServiceView.String())
	viper.SetDefault("pdp.chain_id", preset.SmartContracts.ChainID.String())
	viper.SetDefault("pdp.payer_address", preset.SmartContracts.PayerAddress.String())

	return nil
}

func fullServer(cmd *cobra.Command, _ []string) error {
	// Apply network presets before loading config, but only for flags that weren't explicitly set
	if err := loadPresets(); err != nil {
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

	if err := initTelemetry(
		cmd.Context(),
		appCfg.Identity.Signer.DID().String(),
		userCfg.Network,
		appCfg.Storage.DataDir,
		appCfg.Telemetry,
	); err != nil {
		return fmt.Errorf("initializing telemetry: %w", err)
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

					// Record server metadata
					if err := telemetry.RecordServerInfo(otel.GetMeterProvider().Meter("github."+
						"com/storacha/piri/cli/serve"),
						ctx,
						"full",
						attribute.String("did", appCfg.Identity.Signer.DID().String()),
						attribute.String("owner_address", appCfg.PDPService.OwnerAddress.String()),
						attribute.String("public_url", appCfg.Server.PublicURL.String()),
						attribute.Int64("proof_set", int64(appCfg.UCANService.ProofSetID)),
					); err != nil {
						log.Warnw("Failed to record server info", "error", err)
					}
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

func initTelemetry(ctx context.Context, instanceID, network string, dataDir string, cfg appconfig.TelemetryConfig) error {
	// bail if this has been disabled globally.
	// backwards compatible env var
	if os.Getenv("PIRI_DISABLE_ANALYTICS") != "" {
		return nil
	}
	if cfg.DisableStorachaAnalytics {
		return nil
	}

	t, err := telemetry.Setup(ctx, network, instanceID)
	if err != nil {
		return fmt.Errorf("setting up telemetry: %w", err)
	}

	if err := telemetry.StartHostMetrics(
		ctx,
		t.Metrics.Meter("github.com/storacha/piri/cli/serve"),
		dataDir,
	); err != nil {
		return fmt.Errorf("setting up telemetry host metrics: %w", err)
	}
	return nil
}
