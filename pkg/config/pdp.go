package config

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"net/url"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/storacha/piri/pkg/config/app"
)

type ContractAddresses struct {
	Verifier         string `mapstructure:"verifier" flag:"verifier-address" toml:"verifier,omitempty"`
	ProviderRegistry string `mapstructure:"provider_registry" flag:"provider-registry-address" toml:"provider_registry,omitempty"`
	Service          string `mapstructure:"service" flag:"service-address" toml:"service,omitempty"`
	ServiceView      string `mapstructure:"service_view" flag:"service-view-address" toml:"service_view,omitempty"`
}

type PDPServiceConfig struct {
	OwnerAddress         string               `mapstructure:"owner_address" validate:"required" flag:"owner-address" toml:"owner_address"`
	LotusEndpoint        string               `mapstructure:"lotus_endpoint" validate:"required" flag:"lotus-endpoint" toml:"lotus_endpoint"`
	SigningServiceConfig SigningServiceConfig `mapstructure:"signing_service" toml:"signing_service,omitempty"`
	Contracts            ContractAddresses    `mapstructure:"contracts" toml:"contracts,omitempty"`
	ChainID              string               `mapstructure:"chain_id" toml:"chain_id,omitempty"`
	PayerAddress         string               `mapstructure:"payer_address" validate:"required" toml:"payer_address"`
}

func (c PDPServiceConfig) Validate() error {
	return validateConfig(c)
}

func (c PDPServiceConfig) ToAppConfig() (app.PDPServiceConfig, error) {
	if !common.IsHexAddress(c.OwnerAddress) {
		return app.PDPServiceConfig{}, fmt.Errorf("invalid owner address: %s", c.OwnerAddress)
	}
	lotusEndpoint, err := url.Parse(c.LotusEndpoint)
	if err != nil {
		return app.PDPServiceConfig{}, fmt.Errorf("invalid lotus endpoint: %s: %w", c.LotusEndpoint, err)
	}
	signingServiceConfig, err := c.SigningServiceConfig.ToAppConfig()
	if err != nil {
		return app.PDPServiceConfig{}, fmt.Errorf("invalid signing service config: %s", err)
	}

	// Parse contract addresses if provided
	contracts := app.ContractAddresses{}
	if c.Contracts.Verifier != "" {
		if !common.IsHexAddress(c.Contracts.Verifier) {
			return app.PDPServiceConfig{}, fmt.Errorf("invalid verifier address: %s", c.Contracts.Verifier)
		}
		contracts.Verifier = common.HexToAddress(c.Contracts.Verifier)
	}
	if c.Contracts.ProviderRegistry != "" {
		if !common.IsHexAddress(c.Contracts.ProviderRegistry) {
			return app.PDPServiceConfig{}, fmt.Errorf("invalid provider registry address: %s", c.Contracts.ProviderRegistry)
		}
		contracts.ProviderRegistry = common.HexToAddress(c.Contracts.ProviderRegistry)
	}
	if c.Contracts.Service != "" {
		if !common.IsHexAddress(c.Contracts.Service) {
			return app.PDPServiceConfig{}, fmt.Errorf("invalid service address: %s", c.Contracts.Service)
		}
		contracts.Service = common.HexToAddress(c.Contracts.Service)
	}
	if c.Contracts.ServiceView != "" {
		if !common.IsHexAddress(c.Contracts.ServiceView) {
			return app.PDPServiceConfig{}, fmt.Errorf("invalid service view address: %s", c.Contracts.ServiceView)
		}
		contracts.ServiceView = common.HexToAddress(c.Contracts.ServiceView)
	}

	// Parse ChainID if provided
	var chainID *big.Int
	if c.ChainID != "" {
		chainID = new(big.Int)
		_, ok := chainID.SetString(c.ChainID, 10)
		if !ok {
			return app.PDPServiceConfig{}, fmt.Errorf("invalid chain ID: %s", c.ChainID)
		}
	}

	if !common.IsHexAddress(c.PayerAddress) {
		return app.PDPServiceConfig{}, fmt.Errorf("invalid payer address: %s", c.PayerAddress)
	}

	return app.PDPServiceConfig{
		OwnerAddress:         common.HexToAddress(c.OwnerAddress),
		LotusEndpoint:        lotusEndpoint,
		SigningServiceConfig: signingServiceConfig,
		Contracts:            contracts,
		ChainID:              chainID,
		PayerAddress:         common.HexToAddress(c.PayerAddress),
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
