package smartcontracts

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/storacha/filecoin-services/go/bindings"
)

// ServiceValidator provides methods for payment validation on the main service contract.
// This is separate from the Service interface which uses the view contract.
type ServiceValidator interface {
	// ValidatePayment returns the actual settlement amount after proof reduction.
	// It calls the validatePayment function on the FilecoinWarmStorageService contract
	// which reduces payments proportionally based on proven epochs.
	ValidatePayment(ctx context.Context, railId, proposedAmount, fromEpoch, toEpoch *big.Int) (*ValidationResult, error)

	// Address returns the service contract address
	Address() common.Address
}

// ValidationResult holds the result of a validatePayment call
type ValidationResult struct {
	ModifiedAmount *big.Int // actual amount after proof reduction
	SettleUpTo     *big.Int // epoch to settle up to
	Note           string   // any error/info message from the validator
}

// serviceValidator wraps the main FilecoinWarmStorageService contract for validation calls
type serviceValidator struct {
	address  common.Address
	contract *bindings.FilecoinWarmStorageServiceCaller
}

// NewServiceValidator creates a new service validator wrapping the main service contract
func NewServiceValidator(address common.Address, client bind.ContractBackend) (ServiceValidator, error) {
	contract, err := bindings.NewFilecoinWarmStorageServiceCaller(address, client)
	if err != nil {
		return nil, fmt.Errorf("failed to bind service contract at %s: %w", address, err)
	}

	return &serviceValidator{
		address:  address,
		contract: contract,
	}, nil
}

// ValidatePayment calls the validatePayment function on the service contract
func (v *serviceValidator) ValidatePayment(ctx context.Context, railId, proposedAmount, fromEpoch, toEpoch *big.Int) (*ValidationResult, error) {
	// The 5th argument (rate) is not used by the validator, pass 0
	result, err := v.contract.ValidatePayment(&bind.CallOpts{Context: ctx}, railId, proposedAmount, fromEpoch, toEpoch, big.NewInt(0))
	if err != nil {
		return nil, fmt.Errorf("validatePayment call failed: %w", err)
	}

	return &ValidationResult{
		ModifiedAmount: result.ModifiedAmount,
		SettleUpTo:     result.SettleUpto,
		Note:           result.Note,
	}, nil
}

// Address returns the service contract address
func (v *serviceValidator) Address() common.Address {
	return v.address
}
