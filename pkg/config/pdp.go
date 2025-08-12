package config

import (
	"fmt"
	"net/url"

	"github.com/ethereum/go-ethereum/common"

	"github.com/storacha/piri/pkg/config/app"
)

type LocalPDPConfig struct {
	OwnerAddress    string `mapstructure:"owner_address" validate:"required"`
	ContractAddress string `mapstructure:"contract_address" validate:"required"`
	LotusEndpoint   string `mapstructure:"lotus_endpoint" validate:"required"`
	ProofSetID      uint64 `mapstructure:"proof_set_id" validate:"required"`
}

func (c *LocalPDPConfig) Validate() error {
	return validateConfig(c)
}

func (c *LocalPDPConfig) ToAppConfig() (app.LocalPDPServiceConfig, error) {
	if !common.IsHexAddress(c.OwnerAddress) {
		return app.LocalPDPServiceConfig{}, fmt.Errorf("invalid owner address: %s", c.ContractAddress)
	}
	if !common.IsHexAddress(c.ContractAddress) {
		return app.LocalPDPServiceConfig{}, fmt.Errorf("invalid contract address: %s", c.ContractAddress)
	}
	lotusEndpoint, err := url.Parse(c.LotusEndpoint)
	if err != nil {
		return app.LocalPDPServiceConfig{}, fmt.Errorf("invalid lotus endpoint: %s: %w", c.LotusEndpoint, err)
	}
	return app.LocalPDPServiceConfig{
		OwnerAddress:    common.HexToAddress(c.OwnerAddress),
		ContractAddress: common.HexToAddress(c.ContractAddress),
		LotusEndpoint:   lotusEndpoint,
	}, nil
}

type RemotePDPConfig struct {
	Endpoint string
	ProofSet uint64
}

func (c *RemotePDPConfig) Validate() error {
	return validateConfig(c)
}

func (c *RemotePDPConfig) ToAppConfig() (*app.RemotePDPServiceConfig, error) {
	pdpServerEndpoint, err := url.Parse(c.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid pdp server endpoint: %s: %w", c.Endpoint, err)
	}

	return &app.RemotePDPServiceConfig{
		Endpoint: pdpServerEndpoint,
		ProofSet: c.ProofSet,
	}, nil
}
