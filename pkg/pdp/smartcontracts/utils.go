package smartcontracts

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/storacha/piri/pkg/pdp/smartcontracts/bindings"
)

// GetProvingScheduleFromListener gets the PDPProvingSchedule from a FilecoinWarmStorageService listener.
// Since FilecoinWarmStorageService doesn't directly implement IPDPProvingSchedule,
// this function retrieves the view contract address which does implement it.
func GetProvingScheduleFromListener(listenerAddr common.Address, backend bind.ContractBackend) (PDPProvingSchedule, error) {
	/*
		// The listener should always be a FilecoinWarmStorageService
		warmStorageService, err := bindings.NewFilecoinWarmStorageService(listenerAddr, backend)
		if err != nil {
			return nil, fmt.Errorf("failed to bind FilecoinWarmStorageService at address %s: %w", listenerAddr.Hex(), err)
		}

		// Get the view contract address (FilecoinWarmStorageServiceStateView)
		viewAddr, err := warmStorageService.ViewContractAddress(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get view contract address: %w", err)
		}

		if viewAddr == (common.Address{}) {
			return nil, fmt.Errorf("view contract address is not set for listener at %s", listenerAddr.Hex())
		}


	*/
	// Create and return the PDPProvingSchedule binding to the view contract
	// The view contract (FilecoinWarmStorageServiceStateView) implements IPDPProvingSchedule
	provingSchedule, err := bindings.NewPDPProvingSchedule(listenerAddr, backend)
	if err != nil {
		return nil, fmt.Errorf("failed to create proving schedule binding at view contract address %s: %w", listenerAddr.Hex(), err)
	}

	return provingSchedule, nil
}
