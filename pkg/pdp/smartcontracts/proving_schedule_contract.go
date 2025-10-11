package smartcontracts

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/storacha/piri/pkg/pdp/smartcontracts/bindings"
)

// ContractProvingSchedule implements ProvingScheduleProvider using a real service contract.
type ContractProvingSchedule struct {
	listenerAddr common.Address
	ethClient    bind.ContractBackend
}

// NewContractProvingSchedule creates a new contract-based proving schedule provider.
func NewContractProvingSchedule(listenerAddr common.Address, ethClient bind.ContractBackend) (*ContractProvingSchedule, error) {
	if listenerAddr == (common.Address{}) {
		return nil, fmt.Errorf("listener address cannot be zero")
	}
	if ethClient == nil {
		return nil, fmt.Errorf("eth client cannot be nil")
	}

	return &ContractProvingSchedule{
		listenerAddr: listenerAddr,
		ethClient:    ethClient,
	}, nil
}

// GetPDPConfig retrieves proving configuration from the service contract.
func (c *ContractProvingSchedule) GetPDPConfig(ctx context.Context) (PDPConfig, error) {
	// 1. Get view contract address from listener
	warmStorageService, err := bindings.NewFilecoinWarmStorageService(c.listenerAddr, c.ethClient)
	if err != nil {
		return PDPConfig{}, fmt.Errorf("failed to bind service contract: %w", err)
	}

	viewAddr, err := warmStorageService.ViewContractAddress(&bind.CallOpts{Context: ctx})
	if err != nil {
		return PDPConfig{}, fmt.Errorf("failed to get view contract address: %w", err)
	}

	// 2. Bind to FilecoinWarmStorageServiceStateView
	viewContract, err := bindings.NewFilecoinWarmStorageServiceStateView(viewAddr, c.ethClient)
	if err != nil {
		return PDPConfig{}, fmt.Errorf("failed to bind view contract: %w", err)
	}

	// 3. Call GetPDPConfig on the view contract
	result, err := viewContract.GetPDPConfig(&bind.CallOpts{Context: ctx})
	if err != nil {
		return PDPConfig{}, fmt.Errorf("failed to get PDP config: %w", err)
	}

	// 4. Return the config values
	return PDPConfig{
		MaxProvingPeriod:         uint64(result.MaxProvingPeriod),
		ChallengeWindow:          result.ChallengeWindowSize,
		ChallengesPerProof:       result.ChallengesPerProof,
		InitChallengeWindowStart: result.InitChallengeWindowStart,
	}, nil
}

// NextPDPChallengeWindowStart retrieves the next challenge window start from the service contract.
func (c *ContractProvingSchedule) NextPDPChallengeWindowStart(ctx context.Context, setId *big.Int) (*big.Int, error) {
	// 1. Get view contract address from listener
	warmStorageService, err := bindings.NewFilecoinWarmStorageService(c.listenerAddr, c.ethClient)
	if err != nil {
		return nil, fmt.Errorf("failed to bind service contract: %w", err)
	}

	viewAddr, err := warmStorageService.ViewContractAddress(&bind.CallOpts{Context: ctx})
	if err != nil {
		return nil, fmt.Errorf("failed to get view contract address: %w", err)
	}

	// 2. Bind to FilecoinWarmStorageServiceStateView
	viewContract, err := bindings.NewFilecoinWarmStorageServiceStateView(viewAddr, c.ethClient)
	if err != nil {
		return nil, fmt.Errorf("failed to bind view contract: %w", err)
	}

	// 3. Call NextPDPChallengeWindowStart on the view contract
	return viewContract.NextPDPChallengeWindowStart(&bind.CallOpts{Context: ctx}, setId)
}

// Ensure ContractProvingSchedule implements ProvingScheduleProvider
var _ ProvingScheduleProvider = (*ContractProvingSchedule)(nil)
