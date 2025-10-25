package config

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/storacha/piri/pkg/config/app"
)

type PDPServiceConfig struct {
	OwnerAddress         string               `mapstructure:"owner_address" validate:"required" flag:"owner-address" toml:"owner_address"`
	ContractAddress      string               `mapstructure:"contract_address" validate:"required" flag:"contract-address" toml:"contract_address"`
	LotusEndpoint        string               `mapstructure:"lotus_endpoint" validate:"required" flag:"lotus-endpoint" toml:"lotus_endpoint"`
	SigningServiceConfig SigningServiceConfig `mapstructure:"signing_service" toml:"signing_service,omitempty"`
}

func (c PDPServiceConfig) Validate() error {
	return validateConfig(c)
}

func (c PDPServiceConfig) ToAppConfig() (app.PDPServiceConfig, error) {
	if !common.IsHexAddress(c.OwnerAddress) {
		return app.PDPServiceConfig{}, fmt.Errorf("invalid owner address: %s", c.ContractAddress)
	}
	if !common.IsHexAddress(c.ContractAddress) {
		return app.PDPServiceConfig{}, fmt.Errorf("invalid contract address: %s", c.ContractAddress)
	}
	lotusEndpoint, err := url.Parse(c.LotusEndpoint)
	if err != nil {
		return app.PDPServiceConfig{}, fmt.Errorf("invalid lotus endpoint: %s: %w", c.LotusEndpoint, err)
	}
	signingServiceConfig, err := c.SigningServiceConfig.ToAppConfig()
	if err != nil {
		return app.PDPServiceConfig{}, fmt.Errorf("invalid signing service config: %s", err)
	}
	return app.PDPServiceConfig{
		OwnerAddress:         common.HexToAddress(c.OwnerAddress),
		ContractAddress:      common.HexToAddress(c.ContractAddress),
		LotusEndpoint:        lotusEndpoint,
		SigningServiceConfig: signingServiceConfig,
	}, nil
}

// SigningServiceConfig configures the signing service for PDP operations
type SigningServiceConfig struct {
	// URL endpoint for remote signing service (if using HTTP client)
	Endpoint string `mapstructure:"endpoint" toml:"endpoint,omitempty"`
	// Private key for in-process signing (if using local signer)
	// This should be a hex-encoded private key string
	// NB: this should only be used for development purposes
	PrivateKey string `mapstructure:"private_key" toml:"private_key,omitempty"`
}

func (c SigningServiceConfig) Validate() error {
	return validateConfig(c)
}

func (c SigningServiceConfig) ToAppConfig() (app.SigningServiceConfig, error) {
	// one and only one must be set
	if c.PrivateKey == "" && c.Endpoint == "" {
		return app.SigningServiceConfig{}, fmt.Errorf("signing service requires private_key or endpoint")
	}
	if c.PrivateKey != "" && c.Endpoint != "" {
		return app.SigningServiceConfig{}, fmt.Errorf("signing service private_key and endpoint are mutually exclusive")
	}

	if c.Endpoint != "" {
		ep, err := url.Parse(c.Endpoint)
		if err != nil {
			return app.SigningServiceConfig{}, fmt.Errorf("invalid signing service endpoint: %s: %w", c.Endpoint, err)
		}

		return app.SigningServiceConfig{
			Endpoint: ep,
		}, nil
	} else {
		// we should only use this for development and local testing.
		privateKeyHex := strings.TrimPrefix(c.PrivateKey, "0x")
		privateKeyBytes, err := hex.DecodeString(privateKeyHex)
		if err != nil {
			return app.SigningServiceConfig{}, fmt.Errorf("failed to decode private key: %w", err)
		}

		privateKey, err := crypto.ToECDSA(privateKeyBytes)
		if err != nil {
			return app.SigningServiceConfig{}, fmt.Errorf("failed to parse private key: %w", err)
		}
		log.Warn("signing service operating with local key")
		return app.SigningServiceConfig{
			PrivateKey: privateKey,
		}, nil
	}
}
