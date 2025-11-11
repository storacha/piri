package config

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/storacha/piri/pkg/config/app"
)

type PDPServiceConfig struct {
	OwnerAddress         string               `mapstructure:"owner_address" validate:"required" flag:"owner-address" toml:"owner_address"`
	ContractAddress      string               `mapstructure:"contract_address" validate:"required" flag:"contract-address" toml:"contract_address"`
	LotusEndpoint        string               `mapstructure:"lotus_endpoint" validate:"required" flag:"lotus-endpoint" toml:"lotus_endpoint"`
	SigningServiceConfig SigningServiceConfig `mapstructure:"signing_service" toml:"signing_service,omitempty"`
	AggregationService   AggregationConfig    `mapstructure:"aggregation_service" toml:"aggregation_service,omitempty"`
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
		Aggregation:          c.AggregationService.ToAppConfig(),
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

type AggregationConfig struct {
	CommpCalculator  CommpCalculatorConfig  `mapstructure:"commp_calculator"`
	Aggregator       AggregatorConfig       `mapstructure:"aggregator"`
	AggregateManager AggregateManagerConfig `mapstructure:"aggregate_manager"`
}

func (c AggregationConfig) ToAppConfig() app.AggregationConfig {
	// TODO at a later time we can uncomment this and replace the hard coded default values
	/*
		return app.AggregationConfig{
			CommpCalculator:  c.CommpCalculator.ToAppConfig(),
			Aggregator:       c.Aggregator.ToAppConfig(),
			AggregateManager: c.AggregateManager.ToAppConfig(),
		}
	*/

	return app.AggregationConfig{
		CommpCalculator: app.CommpCalculatorConfig{
			Queue: app.QueueConfig{
				Retries:          10,
				Workers:          2,
				RetryDelay:       30 * time.Second,
				ExtensionTimeout: 5 * time.Second,
			},
		},
		Aggregator: app.AggregatorConfig{
			Queue: app.QueueConfig{
				Retries:          10,
				Workers:          2,
				RetryDelay:       30 * time.Second,
				ExtensionTimeout: 5 * time.Second,
			},
		},
		AggregateManager: app.AggregateManagerConfig{
			Queue: app.QueueConfig{
				Retries:          50,
				Workers:          1, //NB(forrest): must be one until AddRoots supports non-sequential IDs
				RetryDelay:       2 * time.Minute,
				ExtensionTimeout: 4 * time.Minute,
			},
			PollInterval: 30 * time.Second,
			BatchSize:    10,
		},
	}
}

type CommpCalculatorConfig struct {
	Queue QueueConfig `mapstructure:"queue" toml:"queue,omitempty"`
}

func (c CommpCalculatorConfig) ToAppConfig() app.CommpCalculatorConfig {
	return app.CommpCalculatorConfig{
		Queue: c.Queue.ToAppConfig(),
	}
}

type AggregatorConfig struct {
	Queue QueueConfig `mapstructure:"queue" toml:"queue,omitempty"`
}

func (c AggregatorConfig) ToAppConfig() app.AggregatorConfig {
	return app.AggregatorConfig{
		Queue: c.Queue.ToAppConfig(),
	}
}

type AggregateManagerConfig struct {
	Queue        QueueConfig   `mapstructure:"queue" toml:"queue,omitempty"`
	PollInterval time.Duration `mapstructure:"poll_interval" toml:"poll_interval,omitempty"`
	BatchSize    uint          `mapstructure:"batch_size" toml:"batch_size,omitempty"`
}

func (c AggregateManagerConfig) ToAppConfig() app.AggregateManagerConfig {
	return app.AggregateManagerConfig{
		Queue:        c.Queue.ToAppConfig(),
		PollInterval: c.PollInterval,
		BatchSize:    c.BatchSize,
	}
}

type QueueConfig struct {
	Retries          uint          `mapstructure:"retries" toml:"retries,omitempty"`
	Workers          uint          `mapstructure:"workers" toml:"workers,omitempty"`
	RetryDelay       time.Duration `mapstructure:"retry_delay" toml:"retry_delay,omitempty"`
	ExtensionTimeout time.Duration `mapstructure:"extension_timeout" toml:"extension_timeout,omitempty"`
}

func (q QueueConfig) ToAppConfig() app.QueueConfig {
	return app.QueueConfig{
		Retries:          q.Retries,
		Workers:          q.Workers,
		RetryDelay:       q.RetryDelay,
		ExtensionTimeout: q.ExtensionTimeout,
	}
}
