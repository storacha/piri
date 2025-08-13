package cli

import (
	"context"
	"os"
	"strings"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/storacha/piri/cmd/cli/client"
	"github.com/storacha/piri/cmd/cli/delegate"
	"github.com/storacha/piri/cmd/cli/flags"
	"github.com/storacha/piri/cmd/cli/identity"
	"github.com/storacha/piri/cmd/cli/serve"
	"github.com/storacha/piri/cmd/cli/wallet"
	"github.com/storacha/piri/pkg/build"
	"github.com/storacha/piri/pkg/telemetry"
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

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
	cobra.OnInitialize(initConfig, initTelemetry)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "warn", "logging level")

	cobra.CheckErr(flags.SetupCoreFlags(rootCmd.PersistentFlags()))
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

	if logLevel != "" {
		ll, err := logging.LevelFromString(logLevel)
		cobra.CheckErr(err)
		logging.SetAllLoggers(ll)
	}

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
