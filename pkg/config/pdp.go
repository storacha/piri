package config

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/did"
	ucan_http "github.com/storacha/go-ucanto/transport/http"

	"github.com/storacha/piri/pkg/config/app"
)

type ContractAddresses struct {
	Verifier         string `mapstructure:"verifier" validate:"required" flag:"verifier-address" toml:"verifier,omitempty"`
	ProviderRegistry string `mapstructure:"provider_registry" validate:"required" flag:"provider-registry-address" toml:"provider_registry,omitempty"`
	Service          string `mapstructure:"service" validate:"required" flag:"service-address" toml:"service,omitempty"`
	ServiceView      string `mapstructure:"service_view" validate:"required" flag:"service-view-address" toml:"service_view,omitempty"`
	Payments         string `mapstructure:"payments" flag:"payments-address" toml:"payments,omitempty"`
	USDFCToken       string `mapstructure:"usdfc_token" flag:"usdfc-token-address" toml:"usdfc_token,omitempty"`
}

type PDPServiceConfig struct {
	OwnerAddress   string               `mapstructure:"owner_address" validate:"required" flag:"owner-address" toml:"owner_address"`
	LotusEndpoint  string               `mapstructure:"lotus_endpoint" validate:"required" flag:"lotus-endpoint" toml:"lotus_endpoint"`
	SigningService SigningServiceConfig `mapstructure:"signing_service" validate:"required" toml:"signing_service,omitempty"`
	Contracts      ContractAddresses    `mapstructure:"contracts" validate:"required" toml:"contracts,omitempty"`
	ChainID        string               `mapstructure:"chain_id" validate:"required" flag:"chain-id" toml:"chain_id,omitempty"`
	PayerAddress   string               `mapstructure:"payer_address" validate:"required" flag:"payer-address" toml:"payer_address,omitempty"`
	Aggregation    AggregationConfig    `mapstructure:"aggregation" toml:"aggregation,omitempty"`
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
	signingServiceConfig, err := c.SigningService.ToAppConfig()
	if err != nil {
		return app.PDPServiceConfig{}, fmt.Errorf("invalid signing service config: %s", err)
	}

	// Parse contract addresses
	if !common.IsHexAddress(c.Contracts.Verifier) {
		return app.PDPServiceConfig{}, fmt.Errorf("invalid verifier address: %s", c.Contracts.Verifier)
	}

	if !common.IsHexAddress(c.Contracts.ProviderRegistry) {
		return app.PDPServiceConfig{}, fmt.Errorf("invalid provider registry address: %s", c.Contracts.ProviderRegistry)
	}

	if !common.IsHexAddress(c.Contracts.Service) {
		return app.PDPServiceConfig{}, fmt.Errorf("invalid service address: %s", c.Contracts.Service)
	}

	if !common.IsHexAddress(c.Contracts.ServiceView) {
		return app.PDPServiceConfig{}, fmt.Errorf("invalid service view address: %s", c.Contracts.ServiceView)
	}

	if c.Contracts.Payments != "" && !common.IsHexAddress(c.Contracts.Payments) {
		return app.PDPServiceConfig{}, fmt.Errorf("invalid payments address: %s", c.Contracts.Payments)
	}

	if c.Contracts.USDFCToken != "" && !common.IsHexAddress(c.Contracts.USDFCToken) {
		return app.PDPServiceConfig{}, fmt.Errorf("invalid USDFC token address: %s", c.Contracts.USDFCToken)
	}

	chainID := new(big.Int)
	_, ok := chainID.SetString(c.ChainID, 10)
	if !ok {
		return app.PDPServiceConfig{}, fmt.Errorf("invalid chain ID: %s", c.ChainID)
	}

	if !common.IsHexAddress(c.PayerAddress) {
		return app.PDPServiceConfig{}, fmt.Errorf("invalid payer address: %s", c.PayerAddress)
	}

	aggregationCfg, err := c.Aggregation.ToAppConfig()
	if err != nil {
		return app.PDPServiceConfig{}, fmt.Errorf("converting aggregation config: %w", err)
	}

	return app.PDPServiceConfig{
		OwnerAddress:   common.HexToAddress(c.OwnerAddress),
		LotusEndpoint:  lotusEndpoint,
		SigningService: signingServiceConfig,
		Contracts: app.ContractAddresses{
			Verifier:         common.HexToAddress(c.Contracts.Verifier),
			ProviderRegistry: common.HexToAddress(c.Contracts.ProviderRegistry),
			Service:          common.HexToAddress(c.Contracts.Service),
			ServiceView:      common.HexToAddress(c.Contracts.ServiceView),
			Payments:         common.HexToAddress(c.Contracts.Payments),
			USDFCToken:       common.HexToAddress(c.Contracts.USDFCToken),
		},
		ChainID:      chainID,
		PayerAddress: common.HexToAddress(c.PayerAddress),
		Aggregation:  aggregationCfg,
	}, nil
}

// SigningServiceConfig configures the signing service for PDP operations
type SigningServiceConfig struct {
	// Identity of the signing service
	DID string `mapstructure:"did" toml:"did,omitempty"`
	// URL endpoint for remote signing service (if using HTTP client)
	URL string `mapstructure:"url" toml:"url,omitempty"`
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
	if c.PrivateKey == "" && (c.URL == "" || c.DID == "") {
		return app.SigningServiceConfig{}, fmt.Errorf("signing service requires private_key or URL+DID")
	}
	if c.PrivateKey != "" && (c.URL != "" || c.DID != "") {
		return app.SigningServiceConfig{}, fmt.Errorf("signing service private_key and URL+DID are mutually exclusive")
	}

	if c.URL != "" && c.DID != "" {
		ep, err := url.Parse(c.URL)
		if err != nil {
			return app.SigningServiceConfig{}, fmt.Errorf("invalid signing service URL: %s: %w", c.URL, err)
		}
		id, err := did.Parse(c.DID)
		if err != nil {
			return app.SigningServiceConfig{}, fmt.Errorf("parsing signing service DID: %s: %w", c.DID, err)
		}

		channel := ucan_http.NewChannel(ep)
		conn, err := client.NewConnection(id, channel)
		if err != nil {
			return app.SigningServiceConfig{}, fmt.Errorf("creating signing service connection: %w", err)
		}

		return app.SigningServiceConfig{
			Connection: conn,
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

// AggregationConfig configures the PDP aggregation system.
type AggregationConfig struct {
	CommP      CommpConfig            `mapstructure:"commp" toml:"commp,omitempty"`
	Aggregator AggregatorConfig       `mapstructure:"aggregator" toml:"aggregator,omitempty"`
	Manager    AggregateManagerConfig `mapstructure:"manager" toml:"manager,omitempty"`
}

type CommpConfig struct {
	JobQueue JobQueueConfig `mapstructure:"job_queue" toml:"job_queue,omitempty"`
}

type AggregatorConfig struct {
	JobQueue JobQueueConfig `mapstructure:"job_queue" toml:"job_queue,omitempty"`
}

type AggregateManagerConfig struct {
	// PollInterval is how often the aggregation manager flushes its buffer.
	PollInterval time.Duration `mapstructure:"poll_interval" toml:"poll_interval,omitempty"`
	// MaxBatchSize is the maximum number of aggregates per batch submission.
	BatchSize uint           `mapstructure:"batch_size" toml:"batch_size,omitempty"`
	JobQueue  JobQueueConfig `mapstructure:"job_queue" toml:"job_queue,omitempty"`
}

func (a AggregateManagerConfig) ToAppConfig() (app.AggregateManagerConfig, error) {
	if a.BatchSize == 0 {
		return app.AggregateManagerConfig{}, fmt.Errorf("batch size must be greater than zero")
	}
	if a.PollInterval == 0 {
		return app.AggregateManagerConfig{}, fmt.Errorf("poll_interval must be greater than zero")
	}

	jqcfg, err := a.JobQueue.ToAppConfig()
	if err != nil {
		return app.AggregateManagerConfig{}, err
	}
	return app.AggregateManagerConfig{
		PollInterval: a.PollInterval,
		BatchSize:    a.BatchSize,
		JobQueue:     jqcfg,
	}, nil
}

type JobQueueConfig struct {
	// The number of jobs the queue can process in parallel.
	Workers uint `mapstructure:"workers" toml:"workers,omitempty"`
	// The number of times a job can be retried before being considered failed.
	Retries uint `mapstructure:"retries" toml:"retries,omitempty"`
	// The duration between successive retries
	RetryDelay time.Duration `mapstructure:"retry_delay" toml:"retry_delay,omitempty"`
}

func (j JobQueueConfig) ToAppConfig() (app.JobQueueConfig, error) {
	if j.Workers == 0 {
		return app.JobQueueConfig{}, fmt.Errorf("job_queue must have at least one worker")
	}
	if j.RetryDelay == 0 {
		return app.JobQueueConfig{}, fmt.Errorf("job_queue retry delay must be greater than zero")
	}
	return app.JobQueueConfig{
		Workers:    j.Workers,
		Retries:    j.Retries,
		RetryDelay: j.RetryDelay,
	}, nil
}

func (c AggregationConfig) ToAppConfig() (app.AggregationConfig, error) {
	commpJobQueueCfg, err := c.CommP.JobQueue.ToAppConfig()
	if err != nil {
		return app.AggregationConfig{}, err
	}
	aggregatorJobQueueCfg, err := c.Aggregator.JobQueue.ToAppConfig()
	if err != nil {
		return app.AggregationConfig{}, err
	}
	managerCfg, err := c.Manager.ToAppConfig()
	if err != nil {
		return app.AggregationConfig{}, err
	}
	return app.AggregationConfig{
		CommP: app.CommpConfig{
			JobQueue: commpJobQueueCfg,
		},
		Aggregator: app.AggregatorConfig{
			JobQueue: aggregatorJobQueueCfg,
		},
		Manager: managerCfg,
	}, nil
}
