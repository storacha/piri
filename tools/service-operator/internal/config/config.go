package config

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
)

const (
	NetworkCalibration = "calibration"
	NetworkMainnet     = "mainnet"
)

const (
	DefaultRpcUrl          = "https://api.calibration.node.glif.io/rpc/v1"
	DefaultContractAddress = "0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91"
	DefaultPaymentAddress  = "0x6dB198201F900c17e86D267d7Df82567FB03df5E"
	DefaultTokenAddress    = "0xb3042734b608a1B16e9e86B374A3f3e389B4cDf0"
	DefaultNetwork         = NetworkCalibration
)

type Config struct {
	RPCUrl                  string
	ContractAddress         string
	PaymentsContractAddress string
	TokenContractAddress    string
	PrivateKeyPath          string
	KeystorePath            string
	KeystorePassword        string
	Network                 string
}

// NetworkDefaults returns default RPC URL and contract addresses for a given network
func NetworkDefaults(network string) (rpcURL string, contractAddr string, paymentsAddr string, tokenAddr string, err error) {
	switch network {
	case NetworkCalibration:
		return DefaultRpcUrl,
			smartcontracts.Addresses().PDPService.Hex(), // DefaultContractAddress
			DefaultPaymentAddress, // Payments contract on calibration
			DefaultTokenAddress, // Token address must be provided by user
			nil
	case NetworkMainnet:
		return "", "", "", "", fmt.Errorf("mainnet network not yet supported")
	default:
		return "", "", "", "", fmt.Errorf("unknown network: %s (supported: calibration, mainnet)", network)
	}
}

// Validate checks that required configuration fields are set
func (c *Config) Validate() error {
	if c.RPCUrl == "" {
		return fmt.Errorf("rpc-url is required")
	}

	if c.ContractAddress == "" {
		return fmt.Errorf("contract-address is required")
	}

	if !common.IsHexAddress(c.ContractAddress) {
		return fmt.Errorf("invalid contract address: %s", c.ContractAddress)
	}

	if c.PrivateKeyPath == "" && c.KeystorePath == "" {
		return fmt.Errorf("either private-key or keystore must be provided")
	}

	if c.KeystorePath != "" && c.KeystorePassword == "" {
		return fmt.Errorf("keystore-password is required when using keystore")
	}

	return nil
}

// ContractAddr returns the contract address as a common.Address
func (c *Config) ContractAddr() common.Address {
	return common.HexToAddress(c.ContractAddress)
}

// PaymentsAddr returns the payments contract address as a common.Address
func (c *Config) PaymentsAddr() common.Address {
	return common.HexToAddress(c.PaymentsContractAddress)
}

// TokenAddr returns the token contract address as a common.Address
func (c *Config) TokenAddr() common.Address {
	return common.HexToAddress(c.TokenContractAddress)
}
