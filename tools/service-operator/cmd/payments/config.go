package payments

import (
	"github.com/spf13/viper"
	"github.com/storacha/piri/tools/service-operator/internal/config"
)

func loadConfig() (config.Config, error) {
	return config.Config{
		RPCUrl:                  viper.GetString("rpc_url"),
		ContractAddress:         viper.GetString("contract_address"),
		PaymentsContractAddress: viper.GetString("payments_address"),
		TokenContractAddress:    viper.GetString("token_address"),
		PrivateKeyPath:          viper.GetString("private_key"),
		KeystorePath:            viper.GetString("keystore"),
		KeystorePassword:        viper.GetString("keystore_password"),
	}, nil
}
