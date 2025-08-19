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
	OwnerAddress    string `mapstructure:"owner_address" validate:"required" flag:"owner-address"`
	ContractAddress string `mapstructure:"contract_address" validate:"required" flag:"contract-address"`
	LotusEndpoint   string `mapstructure:"lotus_endpoint" validate:"required" flag:"lotus-endpoint"`
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
	}, nil
}
