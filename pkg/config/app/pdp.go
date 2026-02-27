package app

import (
	"crypto/ecdsa"
	"math/big"
	"net/url"
	"runtime"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/storacha/go-ucanto/client"
)

type ContractAddresses struct {
	Verifier         common.Address
	ProviderRegistry common.Address
	Service          common.Address
	ServiceView      common.Address
	Payments         common.Address
	USDFCToken       common.Address
}

type PDPServiceConfig struct {
	// Users address, which owns a proof set and sends messages to the ContractAddress
	OwnerAddress common.Address
	// The URL endpoint of a lotus node used for interaction with chain state.
	LotusEndpoint *url.URL
	// Signing service configuration used to sign PDP operations
	SigningService SigningServiceConfig
	// Smart contract addresses
	Contracts ContractAddresses
	// Filecoin chain ID (314 for mainnet, 314159 for calibration)
	ChainID *big.Int
	// PayerAddress is the Storacha Owned address that pays SPs
	PayerAddress common.Address
	// Aggregation contains aggregation manager configuration
	Aggregation AggregationConfig
}

// SigningServiceConfig configures the signing service for PDP operations
type SigningServiceConfig struct {
	// Connection to the signing service backend.
	Connection client.Connection
	// Private key for in-process signing (if using local signer)
	// NB: this should only be used for development purposes
	PrivateKey *ecdsa.PrivateKey
}

// AggregationConfig configures the PDP aggregation system.
type AggregationConfig struct {
	CommP      CommpConfig
	Aggregator AggregatorConfig
	Manager    AggregateManagerConfig
}

type CommpConfig struct {
	JobQueue JobQueueConfig
}

type AggregatorConfig struct {
	JobQueue JobQueueConfig
}

type AggregateManagerConfig struct {
	// PollInterval is how often the aggregation manager flushes its buffer.
	PollInterval time.Duration
	// MaxBatchSize is the maximum number of aggregates per batch submission.
	BatchSize uint
	JobQueue  JobQueueConfig
}
type JobQueueConfig struct {
	// The number of jobs the queue can process in parallel.
	Workers uint
	// The number of times a job can be retried before being considered failed.
	Retries uint
	// The duration between successive retries
	RetryDelay time.Duration
}

// DefaultJobQueueConfig returns a JobQueueConfig with sensible defaults.
func DefaultJobQueueConfig() JobQueueConfig {
	return JobQueueConfig{
		Workers:    uint(runtime.NumCPU()),
		Retries:    50,
		RetryDelay: 10 * time.Second,
	}
}

// DefaultAggregateManagerConfig returns an AggregateManagerConfig with sensible defaults.
func DefaultAggregateManagerConfig() AggregateManagerConfig {
	return AggregateManagerConfig{
		PollInterval: 30 * time.Second,
		BatchSize:    10,
		JobQueue: JobQueueConfig{
			Workers:    3,
			Retries:    50,
			RetryDelay: time.Minute,
		},
	}
}

// DefaultAggregationConfig returns an AggregationConfig with sensible defaults.
func DefaultAggregationConfig() AggregationConfig {
	return AggregationConfig{
		CommP:      CommpConfig{JobQueue: DefaultJobQueueConfig()},
		Aggregator: AggregatorConfig{JobQueue: DefaultJobQueueConfig()},
		Manager:    DefaultAggregateManagerConfig(),
	}
}
