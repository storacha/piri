package smartcontracts

import (
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

// GetProvingScheduleFromListener creates a ProvingScheduleProvider for calculating proving periods.
// If listenerAddr is non-zero, returns a ContractProvingSchedule that queries the actual service contract.
// Otherwise, returns a HardcodedProvingSchedule with fixed values.
func GetProvingScheduleFromListener(listenerAddr common.Address, backend bind.ContractBackend, chain ChainAPI) (ProvingScheduleProvider, error) {
	// If listener address is set, use the contract-based proving schedule
	if listenerAddr != (common.Address{}) {
		return NewContractProvingSchedule(listenerAddr, backend)
	}

	// Otherwise fall back to hardcoded schedule
	return NewHardcodedProvingSchedule(chain), nil
}
