package config

import (
	"fmt"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
	"github.com/storacha/piri/pkg/config/app"
)

type PDPServerConfig struct {
	Identity   IdentityConfig   `mapstructure:"identity"`
	Repo       RepoConfig       `mapstructure:"repo"`
	Server     ServerConfig     `mapstructure:"server"`
	PDPService PDPServiceConfig `mapstructure:"pdp"`
}

func (c PDPServerConfig) Validate() error {
	return validateConfig(c)
}

type PDPServiceConfig struct {
	OwnerAddress    string         `mapstructure:"owner_address" validate:"required" flag:"owner-address" toml:"owner_address"`
	ContractAddress string         `mapstructure:"contract_address" validate:"required" flag:"contract-address" toml:"contract_address"`
	LotusEndpoint   string         `mapstructure:"lotus_endpoint" validate:"required" flag:"lotus-endpoint" toml:"lotus_endpoint"`
	Signer          SigningService `mapstructure:"signing_service" validate:"required" toml:"signing_service"`
}

type SigningService struct {
	Enabled                bool   `mapstructure:"enabled"`
	PrivateKey             string `mapstructure:"private_key" validate:"required" flag:"private_key" toml:"private_key"`
	PayerAddress           string `mapstructure:"payer_address" validate:"required" flag:"payer-address" toml:"payer_address"`
	ServiceContractAddress string `mapstructure:"service_contract_address" validate:"required" flag:"service-contract-address" toml:"service_contract_address"`
	ChainID                int    `mapstructure:"chain_id" validate:"required" flag:"chain-id" toml:"chain_id"`
}

func (p PDPServiceConfig) Validate() error {
	return validateConfig(p)
}

func (p PDPServiceConfig) ToAppConfig() (app.PDPServiceConfig, error) {
	if !common.IsHexAddress(p.OwnerAddress) {
		return app.PDPServiceConfig{}, fmt.Errorf("invalid owner address: %s", p.ContractAddress)
	}
	if !common.IsHexAddress(p.ContractAddress) {
		return app.PDPServiceConfig{}, fmt.Errorf("invalid contract address: %s", p.ContractAddress)
	}
	lotusEndpoint, err := url.Parse(p.LotusEndpoint)
	if err != nil {
		return app.PDPServiceConfig{}, fmt.Errorf("invalid lotus endpoint: %s: %w", p.LotusEndpoint, err)
	}
	return app.PDPServiceConfig{
		OwnerAddress:    common.HexToAddress(p.OwnerAddress),
		ContractAddress: common.HexToAddress(p.ContractAddress),
		LotusEndpoint:   lotusEndpoint,
		SigningService: app.SigningServiceConfig{
			Enabled: p.Signer.Enabled,
			// TODO deploy a signing service and us the URL here, though this
			// can also operate with the contract owners private key inprocess for testing
			//Endpoint:               lo.Must(url.Parse("http://localhost:8080")),
			PrivateKey:             p.Signer.PrivateKey,
			PayerAddress:           common.HexToAddress(p.Signer.PayerAddress),
			ServiceContractAddress: common.HexToAddress(p.Signer.ServiceContractAddress),
			ChainID:                int64(p.Signer.ChainID),
		},
	}, nil
}
