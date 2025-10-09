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
	// Fresh deployment on 2025-10-09 for EIP-712 signature testing
	// Deployed from commit 7b28dece8236f63bcdeb7b4359e1062038c9da98
	// See DEPLOYMENT_LOG.md for full details
	return PDPContracts{
		// PDPVerifier proxy contract address (PDP v2.2.1)
		PDPVerifier: common.HexToAddress("0xa31Dc22286442B733B52ac102461A0685Cc5D36f"),
		// ServiceProviderRegistry proxy address - owned by storacha
		ProviderRegistry: common.HexToAddress("0x7F603F206015A4d608a6aBbb275F306fC925D6bD"),
		// FilecoinWarmStorageService proxy address - owned by storacha
		PDPService: common.HexToAddress("0x60F412Fd67908a38A5E05C54905daB923413EEA6"),
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
