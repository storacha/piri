package app

import (
	"crypto/ecdsa"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
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
}

// SigningServiceConfig configures the signing service for PDP operations
type SigningServiceConfig struct {
	// Connection to the signing service backend.
	Connection client.Connection
	// Proof the node can use the signing service to sign PDP operations.
	Proofs delegation.Proofs
	// Private key for in-process signing (if using local signer)
	// NB: this should only be used for development purposes
	PrivateKey *ecdsa.PrivateKey
}
