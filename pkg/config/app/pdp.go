package app

import (
	"crypto/ecdsa"
	"math/big"
	"net/url"

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
}

// SigningServiceConfig configures the signing service for PDP operations
type SigningServiceConfig struct {
	// Connection to the signing service backend.
	Connection client.Connection
	// Private key for in-process signing (if using local signer)
	// NB: this should only be used for development purposes
	PrivateKey *ecdsa.PrivateKey
}
