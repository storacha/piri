package smartcontracts

import (
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

// GetProvingScheduleFromListener creates a ProvingScheduleProvider for calculating proving periods.
// Currently returns a hardcoded implementation that uses fixed proving period values.
//
// TODO: When the FilecoinWarmStorageService contract is integrated, check if listenerAddr
// is non-zero and return ContractProvingSchedule instead:
//
//	if listenerAddr != (common.Address{}) {
//	    return NewContractProvingSchedule(listenerAddr, backend)
//	}
//
// For now, always uses hardcoded values: 60 epoch period, 30 epoch window.
func GetProvingScheduleFromListener(listenerAddr common.Address, backend bind.ContractBackend, chain ChainAPI) (ProvingScheduleProvider, error) {
	// Always use hardcoded schedule for now
	return NewHardcodedProvingSchedule(chain), nil
}
