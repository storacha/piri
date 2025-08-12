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
	ProofSetID    uint64
}
