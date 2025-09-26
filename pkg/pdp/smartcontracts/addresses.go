package smartcontracts

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/snadrus/must"
)

/*

# DEPLOYMENT SUMMARY
PDPVerifier Implementation: 0xe942ac07C1769BC6A4618938e503D8A5341B0E1E
PDPVerifier Proxy: 0x2A08767C44871A3103eAA0019141C9d3756C553e
Payments Contract: 0xC1e2042ee61acEF5B40971Cb2f4AC163De20c33d
ServiceProviderRegistry Implementation: 0x6EAae4EB6c9a98606729ED7C410075d49c6CA311
ServiceProviderRegistry Proxy: 0xc24a73E536979827aa516A68cb279a5092deEA3a
FilecoinWarmStorageService Implementation: 0xCab5c29615f1dCb199Bd2Fc0d5d15E7bDd8234de
FilecoinWarmStorageService Proxy: 0xf58624A1A8c8280a23E5083B01ef7A9488411a0E
FilecoinWarmStorageServiceStateView: 0x982964E760f69D238dB24025027A6fd57C27423a

USDFC token address: 0xb3042734b608a1B16e9e86B374A3f3e389B4cDf0
FilCDN controller address: 0x5f7E5E2A756430EdeE781FF6e6F7954254Ef629A
FilCDN beneficiary address: 0x1D60d2F5960Af6341e842C539985FA297E10d6eA
Max proving period: 30 epochs
Challenge window size: 15 epochs
Service name: PiriStorageContract
Service description: A smartcontract for testing with Piri
*/

const (
	// These are the verifiers used by FilOZ team,
	//we use these instead of our own since they have a dashboard watching them.
	PDPVerifierProxyAddress          = "0x445238Eca6c6aB8Dff1Aa6087d9c05734D22f137"
	PDPVerifierImplementationAddress = " 0x648E8D9103Ec91542DcD0045A65Ef9679F886e82"

	PDPFilecoinWarmStorageServiceRecordKeeperAddress = "0xf58624A1A8c8280a23E5083B01ef7A9488411a0E"

	ServiceProviderRegistryImplementationAddress = "0x6EAae4EB6c9a98606729ED7C410075d49c6CA311"
	ServiceProviderRegistryProxyAddress          = "0xc24a73E536979827aa516A68cb279a5092deEA3a"
)

type PDPContracts struct {
	PDPVerifier   common.Address
	RecordKeepers []common.Address
}

func Addresses() PDPContracts {
	// addresses here based on https://github.com/FilOzone/pdp/?tab=readme-ov-file#contracts
	// NB(forrest): For now, until we are ready to launch a production network we return
	// the PDP Service address of the calibration contract, defined at URL above.
	return PDPContracts{
		// PDPVerifier contract address
		PDPVerifier: common.HexToAddress(PDPVerifierProxyAddress),
		// FilecoinWarmStorageService contract address
		RecordKeepers: []common.Address{
			common.HexToAddress(PDPFilecoinWarmStorageServiceRecordKeeperAddress),
		},
	}
}

const NumChallenges = 5

func SybilFee() *big.Int {
	return must.One(types.ParseFIL("0.1")).Int
}

func RegisterProviderFee() *big.Int {
	return must.One(types.ParseFIL("5")).Int
}
