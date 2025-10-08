package provider

import (
	"github.com/spf13/viper"
	"github.com/storacha/piri/tools/service-operator/internal/config"
)

func loadConfig() (config.Config, error) {
	network := viper.GetString("network")
	rpcURL := viper.GetString("rpc_url")
	contractAddr := viper.GetString("contract_address")

	// If network is specified, use it to set defaults
	if network != "" {
		defaultRPC, defaultAddr, _, _, err := config.NetworkDefaults(network)
		if err != nil {
			return config.Config{}, err
		}

		// Only use defaults if not explicitly set
		if rpcURL == "" {
			rpcURL = defaultRPC
		}
		if contractAddr == "" {
			contractAddr = defaultAddr
		}
	}

	return config.Config{
		RPCUrl:           rpcURL,
		ContractAddress:  contractAddr,
		PrivateKeyPath:   viper.GetString("private_key"),
		KeystorePath:     viper.GetString("keystore"),
		KeystorePassword: viper.GetString("keystore_password"),
		Network:          network,
	}, nil
}
