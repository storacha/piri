package app

import (
	"crypto/ecdsa"
	"math/big"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
)

type ContractAddresses struct {
	Verifier         common.Address
	ProviderRegistry common.Address
	Service          common.Address
	ServiceView      common.Address
}

type PDPServiceConfig struct {
	// Users address, which owns a proof set and sends messages to the ContractAddress
	OwnerAddress common.Address
	// The URL endpoint of a lotus node used for interaction with chain state.
	LotusEndpoint *url.URL
	// Signing service configuration used to sign PDP operations
	SigningServiceConfig SigningServiceConfig
	// Smart contract addresses
	Contracts ContractAddresses
	// Filecoin chain ID (314 for mainnet, 314159 for calibration)
	ChainID *big.Int
	// PayerAddress is the Storacha Owned address that pays SPs
	PayerAddress common.Address
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
