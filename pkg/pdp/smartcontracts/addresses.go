package smartcontracts

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/snadrus/must"
)

type PDPContracts struct {
	PDPVerifier      common.Address
	ProviderRegistry common.Address
	PDPService       common.Address
}

func Addresses() PDPContracts {
	// addresses here based on https://github.com/FilOzone/pdp/?tab=readme-ov-file#contracts
	// NB(forrest): For now, until we are ready to launch a production network we return
	// the PDP Service address of the calibration contract, defined at URL above.
	return PDPContracts{
		// PDPVerifier contract address
		PDPVerifier: common.HexToAddress("0x445238Eca6c6aB8Dff1Aa6087d9c05734D22f137"),
		// This contract and its address are owned by storacha
		ProviderRegistry: common.HexToAddress("0x0aD6636eE10682232320356bc904ab07f8837bB1"),
		// This contract and its address are owned by storacha, and uses ProviderRegistry for membership
		PDPService: common.HexToAddress("0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91"),
	}
}

// NB: definion here: https://github.com/storacha/filecoin-services/blob/main/service_contracts/src/FilecoinWarmStorageService.sol#L23
const NumChallenges = 5

// NB: defintion here: https://github.com/FilOzone/pdp/blob/main/src/Fees.sol#L11
func SybilFee() *big.Int {
	return must.One(types.ParseFIL("0.1")).Int
}

// NB: definition here: https://github.com/storacha/filecoin-services/blob/main/service_contracts/src/ServiceProviderRegistry.sol#L54
func RegisterProviderFee() *big.Int {
	return must.One(types.ParseFIL("5")).Int
}
