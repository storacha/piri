package cmd

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/storacha/piri/tools/service-operator/internal/config"

	"github.com/storacha/piri/tools/service-operator/cmd/payments"
	"github.com/storacha/piri/tools/service-operator/cmd/provider"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "service-operator",
	Short: "Service operator CLI for managing FilecoinWarmStorageService contracts",
	Long: `service-operator is a CLI tool for managing FilecoinWarmStorageService smart contracts.
It provides commands to approve/remove providers, configure service settings, and more.`,
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./service-operator.yaml)")

	rootCmd.PersistentFlags().String("rpc-url", config.DefaultRpcUrl, "Ethereum RPC endpoint URL")
	rootCmd.PersistentFlags().String("contract-address", config.DefaultContractAddress, "FilecoinWarmStorageService contract address")
	rootCmd.PersistentFlags().String("payments-address", config.DefaultPaymentAddress, "Payments contract address")
	rootCmd.PersistentFlags().String("token-address", config.DefaultTokenAddress, "ERC20 token contract address (must support EIP-2612)")
	rootCmd.PersistentFlags().String("network", config.DefaultNetwork, "Network preset (calibration or mainnet)")
	rootCmd.PersistentFlags().String("private-key", "", "Path to private key file")
	rootCmd.PersistentFlags().String("keystore", "", "Path to keystore file (alternative to private-key)")
	rootCmd.PersistentFlags().String("keystore-password", "", "Keystore password")

	cobra.CheckErr(viper.BindPFlag("rpc_url", rootCmd.PersistentFlags().Lookup("rpc-url")))
	cobra.CheckErr(viper.BindPFlag("contract_address", rootCmd.PersistentFlags().Lookup("contract-address")))
	cobra.CheckErr(viper.BindPFlag("payments_address", rootCmd.PersistentFlags().Lookup("payments-address")))
	cobra.CheckErr(viper.BindPFlag("token_address", rootCmd.PersistentFlags().Lookup("token-address")))
	cobra.CheckErr(viper.BindPFlag("private_key", rootCmd.PersistentFlags().Lookup("private-key")))
	cobra.CheckErr(viper.BindPFlag("keystore", rootCmd.PersistentFlags().Lookup("keystore")))
	cobra.CheckErr(viper.BindPFlag("keystore_password", rootCmd.PersistentFlags().Lookup("keystore-password")))
	cobra.CheckErr(viper.BindPFlag("network", rootCmd.PersistentFlags().Lookup("network")))

	rootCmd.AddCommand(provider.Cmd)
	rootCmd.AddCommand(payments.Cmd)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("service-operator")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
	}

	viper.SetEnvPrefix("SERVICE_OPERATOR")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	// Don't error if config file is not found
	_ = viper.ReadInConfig()
}

func Execute(ctx context.Context) error {
	return rootCmd.ExecuteContext(ctx)
}
