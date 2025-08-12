package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/storacha/piri/cmd/cli/client"
	"github.com/storacha/piri/cmd/cli/delegate"
	"github.com/storacha/piri/cmd/cli/identity"
	"github.com/storacha/piri/cmd/cli/serve"
	"github.com/storacha/piri/cmd/cli/wallet"
	"github.com/storacha/piri/pkg/build"
	"github.com/storacha/piri/pkg/telemetry"
)

func ExecuteContext(ctx context.Context) {
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		log.Fatal(err)
	}
}

var log = logging.Logger("cmd")

const piriShortDescription = `
Piri is the software run by all storage providers on the Storacha network
`

const piriLongDescription = `
Piri - Provable Information Retention Interface
Piri can run entirely on its own with no software other than Filecoin Lotus, or it can integrate into Filecoin storage provider operation running Curio.
`

var (
	cfgFile  string
	logLevel string
	rootCmd  = &cobra.Command{
		Use:   "piri",
		Short: piriShortDescription,
		Long:  piriLongDescription,
	}
)

func init() {
	cobra.OnInitialize(initLogging, initConfig, initTelemetry)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "logging level")

	rootCmd.PersistentFlags().String("data-dir", filepath.Join(lo.Must(os.UserHomeDir()), ".storacha"), "Storage service data directory")
	cobra.CheckErr(viper.BindPFlag("repo.data_dir", rootCmd.PersistentFlags().Lookup("data-dir")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("repo.data_dir", "PIRI_DATA_DIR"))

	rootCmd.PersistentFlags().String("temp-dir", filepath.Join(os.TempDir(), "storage"), "Storage service temp directory")
	cobra.CheckErr(viper.BindPFlag("repo.temp_dir", rootCmd.PersistentFlags().Lookup("temp-dir")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("repo.temp_dir", "PIRI_TEMP_DIR"))

	rootCmd.PersistentFlags().String("key-file", "", "Path to a PEM file containing ed25519 private key")
	cobra.CheckErr(rootCmd.MarkPersistentFlagFilename("key-file", "pem"))
	cobra.CheckErr(viper.BindPFlag("identity.key_file", rootCmd.PersistentFlags().Lookup("key-file")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("identity.key_file", "PIRI_KEY_FILE"))

	// register all commands and their subcommands
	rootCmd.AddCommand(serve.Cmd)
	rootCmd.AddCommand(wallet.Cmd)
	rootCmd.AddCommand(identity.Cmd)
	rootCmd.AddCommand(delegate.Cmd)
	rootCmd.AddCommand(client.Cmd)

}

func initConfig() {
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetEnvPrefix("PIRI")

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
		cobra.CheckErr(viper.ReadInConfig())
	}
}

func initTelemetry() {
	// bail if this has been disabled.
	if os.Getenv("PIRI_DISABLE_ANALYTICS") != "" {
		return
	}
	telCfg := telemetry.Config{
		ServiceName:    "piri",
		ServiceVersion: build.Version,
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	if err := telemetry.Initialize(ctx, telCfg); err != nil {
		log.Warnf("failed to initialize telemetry: %s", err)
	}
}

func initLogging() {
	if logLevel != "" {
		ll, err := logging.LevelFromString(logLevel)
		cobra.CheckErr(err)
		logging.SetAllLoggers(ll)
	} else {
		logging.SetLogLevel("database/gorm", "error")
		logging.SetLogLevel("fns", "error")
		logging.SetLogLevel("blobs", "error")
		logging.SetLogLevel("scheduler/chain", "error")
		logging.SetLogLevel("pdp/service", "info")
		logging.SetLogLevel("config", "error")
		logging.SetLogLevel("f3/internal/caching", "error")
		logging.SetLogLevel("rpc", "error")
		logging.SetLogLevel("test-logger", "error")
		logging.SetLogLevel("build/build-types", "error")
		logging.SetLogLevel("merkledag", "error")
		logging.SetLogLevel("pdp/client", "info")
		logging.SetLogLevel("telemetry", "info")
		logging.SetLogLevel("publisher", "warn")
		logging.SetLogLevel("f3/gpbft", "error")
		logging.SetLogLevel("alerting", "error")
		logging.SetLogLevel("blockstore", "error")
		logging.SetLogLevel("eventlog", "error")
		logging.SetLogLevel("indexer/schema", "error")
		logging.SetLogLevel("statetree", "error")
		logging.SetLogLevel("cli/wallet", "info")
		logging.SetLogLevel("httpreader", "error")
		logging.SetLogLevel("blockservice", "error")
		logging.SetLogLevel("pubsub", "error")
		logging.SetLogLevel("gossiptopic", "error")
		logging.SetLogLevel("announce", "warn")
		logging.SetLogLevel("proof", "warn")
		logging.SetLogLevel("pdp/aggregator", "warn")
		logging.SetLogLevel("chainstore", "error")
		logging.SetLogLevel("f3/manifest-provider", "error")
		logging.SetLogLevel("store", "error")
		logging.SetLogLevel("pdp/scheduler", "info")
		logging.SetLogLevel("metrics", "warn")
		logging.SetLogLevel("pdp/tasks", "info")
		logging.SetLogLevel("pdp/api", "info")
		logging.SetLogLevel("replicator", "info")
		logging.SetLogLevel("storage/ucan", "info")
		logging.SetLogLevel("fsutil", "error")
		logging.SetLogLevel("types", "error")
		logging.SetLogLevel("peerstore", "error")
		logging.SetLogLevel("pdp/server", "info")
		logging.SetLogLevel("cmd/serve", "info")
		logging.SetLogLevel("amt", "error")
		logging.SetLogLevel("discovery-backoff", "error")
		logging.SetLogLevel("database", "warn")
		logging.SetLogLevel("principal-resolver", "error")
		logging.SetLogLevel("server", "info")
		logging.SetLogLevel("auth", "error")
		logging.SetLogLevel("rpcenc", "error")
		logging.SetLogLevel("storage", "info")
		logging.SetLogLevel("resources", "error")
	}

}
