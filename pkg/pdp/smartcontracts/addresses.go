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
	// deployed at: https://github.com/storacha/filecoin-services/pull/3
	return PDPContracts{
		// PDPVerifier proxy contract address (PDP v2.2.1)
		PDPVerifier: common.HexToAddress("0xB020524bdE8926cD430A4F79B2AaccFd2694793b"),
		// ServiceProviderRegistry proxy address - owned by storacha
		ProviderRegistry: common.HexToAddress("0x8D0560F93022414e7787207682a8D562de02D62f"),
		// FilecoinWarmStorageService proxy address - owned by storacha
		PDPService: common.HexToAddress("0xB9753937D3Bc1416f7d741d75b1671A1edb3e10A"),
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
