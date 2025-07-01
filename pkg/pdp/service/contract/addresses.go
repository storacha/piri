package contract

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/snadrus/must"
)

type PDPContracts struct {
	PDPVerifier common.Address
}

func Addresses() PDPContracts {
	// addresses here based on https://github.com/FilOzone/pdp/?tab=readme-ov-file#contracts
	// NB(forrest): For now, until we are ready to launch a production network we return
	// the PDP Service address of the calibration contract, defined at URL above.
	return PDPContracts{
		PDPVerifier: common.HexToAddress("0x5A23b7df87f59A291C26A2A1d684AD03Ce9B68DC"),
	}
}

const NumChallenges = 5

func SybilFee() *big.Int {
	return must.One(types.ParseFIL("0.1")).Int
}
