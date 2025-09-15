package cli

import (
	"context"
	"fmt"
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
	"github.com/storacha/piri/cmd/cli/initalize"
	"github.com/storacha/piri/cmd/cli/serve"
	"github.com/storacha/piri/cmd/cli/status"
	"github.com/storacha/piri/cmd/cli/update"
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

var (
	cfgFile  string
	logLevel string
	rootCmd  = &cobra.Command{
		Use:   "piri",
		Short: piriShortDescription,
		Long: fmt.Sprintf(`Piri - Provable Information Retention Interface (Version: %s)
Piri can run entirely on its own with no software other than Filecoin Lotus, or it can integrate into Filecoin storage provider operation running Curio.`, build.Version),
		Version: build.Version,
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
	rootCmd.AddCommand(initalize.InitCmd)
	rootCmd.AddCommand(initalize.InstallCmd)
	rootCmd.AddCommand(client.Cmd)
	rootCmd.AddCommand(status.Cmd)
	rootCmd.AddCommand(update.UpdateCmd)

	// this command is hidden from the user
	rootCmd.AddCommand(update.InternalUpdateCmd)

}

func initConfig() {
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetEnvPrefix("PIRI")

	// if we are provided an explicit config file, use it
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
		cobra.CheckErr(viper.ReadInConfig())
	} else {
		// otherwise look for piri-config.toml in current directory
		viper.SetConfigName("piri-config")
		viper.SetConfigType("toml")
		viper.AddConfigPath(".")
		// Don't error if config file is not found - it's optional
		_ = viper.ReadInConfig()
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
		// else set all loggers to error level, then the ones we care most about to info.
		logging.SetAllLoggers(logging.LevelError)
		logging.SetLogLevel("pdp/service", "info")
		logging.SetLogLevel("pdp/client", "info")
		logging.SetLogLevel("telemetry", "info")
		logging.SetLogLevel("publisher", "warn")
		logging.SetLogLevel("cli/wallet", "info")
		logging.SetLogLevel("announce", "warn")
		logging.SetLogLevel("proof", "warn")
		logging.SetLogLevel("pdp/aggregator", "info")
		logging.SetLogLevel("pdp/scheduler", "info")
		logging.SetLogLevel("metrics", "warn")
		logging.SetLogLevel("pdp/tasks", "info")
		logging.SetLogLevel("pdp/api", "info")
		logging.SetLogLevel("replicator", "info")
		logging.SetLogLevel("storage/ucan", "info")
		logging.SetLogLevel("pdp/server", "info")
		logging.SetLogLevel("cmd/serve", "info")
		logging.SetLogLevel("database", "warn")
		logging.SetLogLevel("server", "info")
		logging.SetLogLevel("storage", "info")
	}

}
