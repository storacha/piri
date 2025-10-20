package smartcontracts

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/snadrus/must"
)

const FilecoinEpoch = 30 * time.Second

// TODO this will either be mainnet or calibnet, currnet ID is calibnet
// we could also pull this from the lotus clinet piri is configued with
var ChainID = big.NewInt(314159)

// PayerAddress is the Storacha Owned address that pays SPs
var PayerAddress = common.HexToAddress("0x8d3d7cE4F43607C9d0ac01f695c7A9caC135f9AD")

type PDPContracts struct {
	Verifier         common.Address
	ProviderRegistry common.Address
	Service          common.Address
	ServiceView      common.Address
}

func Addresses() PDPContracts {
	// addresses here based on https://github.com/FilOzone/pdp/?tab=readme-ov-file#contracts
	// NB(forrest): For now, until we are ready to launch a production network we return
	// the PDP Service address of the calibration contract, defined at URL above.
	return PDPContracts{
		// PDPVerifier contract address
		Verifier: common.HexToAddress("0xB020524bdE8926cD430A4F79B2AaccFd2694793b"),
		// This contract and its address are owned by storacha
		ProviderRegistry: common.HexToAddress("0x8D0560F93022414e7787207682a8D562de02D62f"),
		// This contract and its address are owned by storacha, and uses ProviderRegistry for membership
		Service:     common.HexToAddress("0xB9753937D3Bc1416f7d741d75b1671A1edb3e10A"),
		ServiceView: common.HexToAddress("0xb2eC3e67753F1c05e8B318298Bd0eD89046a3031"),
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
