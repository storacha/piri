package app

import (
	"net/url"

	"github.com/ethereum/go-ethereum/common"
)

type PDPServiceConfig struct {
	// Users address, which owns a proof set and sends messages to the ContractAddress
	OwnerAddress common.Address
	// The 'PDP Service' contract address defined here: https://github.com/FilOzone/pdp/
	ContractAddress common.Address
	// The URL endpoint of a lotus node used for interaction with chain state.
	LotusEndpoint *url.URL
	// Signing service configuration for authenticated operations
	SigningService SigningServiceConfig
}

// SigningServiceConfig configures the signing service for PDP operations
type SigningServiceConfig struct {
	// Enable signing service integration
	Enabled bool
	// URL endpoint for remote signing service (if using HTTP client)
	Endpoint *url.URL
	// Private key for in-process signing (if using local signer)
	// This should be a hex-encoded private key string
	PrivateKey string
	// Address of the payer (the account that pays for storage)
	PayerAddress common.Address
	// Address of the FilecoinWarmStorageService contract
	ServiceContractAddress common.Address
	// Chain ID for EIP-712 signatures
	ChainID int64
}
