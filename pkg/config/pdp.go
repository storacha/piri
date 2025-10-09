package config

import (
	"fmt"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
	"github.com/samber/lo"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
	"github.com/storacha/piri/pkg/presets"

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
	OwnerAddress    string `mapstructure:"owner_address" validate:"required" flag:"owner-address" toml:"owner_address"`
	ContractAddress string `mapstructure:"contract_address" validate:"required" flag:"contract-address" toml:"contract_address"`
	LotusEndpoint   string `mapstructure:"lotus_endpoint" validate:"required" flag:"lotus-endpoint" toml:"lotus_endpoint"`
}

func (c PDPServiceConfig) Validate() error {
	return validateConfig(c)
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
			Enabled:                true,
			Endpoint:               lo.Must(url.Parse("http://localhost:8080")),
			PayerAddress:           presets.StorachaUSDFCAddress,
			ServiceContractAddress: smartcontracts.Addresses().PDPService,
			ChainID:                314159,
		},
	}, nil
}
