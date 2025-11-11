package app

import (
	"crypto/ecdsa"
	"net/url"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type PDPServiceConfig struct {
	// Users address, which owns a proof set and sends messages to the ContractAddress
	OwnerAddress common.Address
	// The 'PDP Service' contract address defined here: https://github.com/FilOzone/pdp/
	ContractAddress common.Address
	// The URL endpoint of a lotus node used for interaction with chain state.
	LotusEndpoint *url.URL
	// Signing service configuration used to sign PDP operations
	SigningServiceConfig SigningServiceConfig
	// Aggregation service configuration for aggregating data for PDP operations
	Aggregation AggregationConfig
}

// SigningServiceConfig configures the signing service for PDP operations
type SigningServiceConfig struct {
	// URL endpoint for remote signing service (if using HTTP client)
	Endpoint *url.URL
	// Private key for in-process signing (if using local signer)
	// This should be a hex-encoded private key string
	// NB: this should only be used for development purposes
	PrivateKey *ecdsa.PrivateKey
}

type AggregationConfig struct {
	CommpCalculator  CommpCalculatorConfig
	Aggregator       AggregatorConfig
	AggregateManager AggregateManagerConfig
}

type CommpCalculatorConfig struct {
	Queue QueueConfig
}

type AggregatorConfig struct {
	Queue QueueConfig
}

type AggregateManagerConfig struct {
	Queue        QueueConfig
	PollInterval time.Duration
	BatchSize    uint
}

type QueueConfig struct {
	Retries          uint
	Workers          uint
	RetryDelay       time.Duration
	ExtensionTimeout time.Duration
}
