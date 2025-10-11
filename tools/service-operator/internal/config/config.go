package config

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

type Config struct {
	RPCUrl                  string
	ContractAddress         string
	PaymentsContractAddress string
	TokenContractAddress    string
	PrivateKeyPath          string
	KeystorePath            string
	KeystorePassword        string
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
