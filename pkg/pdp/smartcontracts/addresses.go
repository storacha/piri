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
		Verifier: common.HexToAddress("0x85e366Cf9DD2c0aE37E963d9556F5f4718d6417C"),
		// This contract and its address are owned by storacha
		ProviderRegistry: common.HexToAddress("0x6A96aaB210B75ee733f0A291B5D8d4A053643979"),
		// This contract and its address are owned by storacha, and uses ProviderRegistry for membership
		Service:     common.HexToAddress("0x0c6875983B20901a7C3c86871f43FdEE77946424"),
		ServiceView: common.HexToAddress("0xEAD67d775f36D1d2894854D20e042C77A3CC20a5"),
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
