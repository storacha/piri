package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"github.com/storacha/filecoin-services/go/bindings"
	"github.com/storacha/piri/pkg/pdp/httpapi/server/middleware"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
	"github.com/storacha/piri/pkg/pdp/types"
)

func (p *PDPService) GetProviderStatus(ctx context.Context) (types.GetProviderStatusResults, error) {
	bindCtx := &bind.CallOpts{Context: ctx}
	registry, err := bindings.NewServiceProviderRegistry(smartcontracts.Addresses().ProviderRegistry, p.contractBackend)
	if err != nil {
		return types.GetProviderStatusResults{}, fmt.Errorf("failed to create service registry binding: %w", err)
	}

	// Check if provider is registered on-chain
	isRegistered, err := registry.IsRegisteredProvider(bindCtx, p.address)
	if err != nil {
		return types.GetProviderStatusResults{}, fmt.Errorf("failed to check if service provider is registered: %w", err)
	}

	result := types.GetProviderStatusResults{
		Address: p.address,
	}

	if isRegistered {
		// Provider is registered, get full info
		providerInfoView, err := registry.GetProviderByAddress(bindCtx, p.address)
		if err != nil {
			return types.GetProviderStatusResults{}, fmt.Errorf("failed to get provider by address: %w", err)
		}

		approved, err := p.viewContract.IsProviderApproved(providerInfoView.ProviderId)
		if err != nil {
			return types.GetProviderStatusResults{}, fmt.Errorf("failed to check if provider is approved: %w", err)
		}
		status := "registered"
		if approved {
			status = "approved"
		}
		result.ID = providerInfoView.ProviderId.Uint64()
		result.Payee = providerInfoView.Info.Payee
		result.IsRegistered = true
		result.IsActive = providerInfoView.Info.IsActive
		result.Name = providerInfoView.Info.Name
		result.Description = providerInfoView.Info.Description
		result.RegistrationStatus = status
		result.IsApproved = approved

		return result, nil
	}

	// Not registered on-chain, check if there's a pending registration
	var pendingReg models.PDPProviderRegistration
	err = p.db.WithContext(ctx).
		Where("service = ? AND provider_registered = ?", p.name, false).
		First(&pendingReg).Error

	if err == nil {
		// Found a pending registration
		result.IsRegistered = false
		result.RegistrationStatus = "pending"
		result.Payee = common.Address{}
		return result, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return types.GetProviderStatusResults{}, fmt.Errorf("failed to check for pending registration: %w", err)
	}

	// No registration found, neither on-chain nor pending
	result.IsRegistered = false
	result.RegistrationStatus = "unregistered"
	result.Payee = common.Address{}

	return result, nil
}

// RequireProviderApproved checks if the provider is both registered and approved.
// Returns a rich contextual error if authorization fails.
func (p *PDPService) RequireProviderApproved(ctx context.Context) error {
	regStatus, err := p.GetProviderStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to check registration status: %w", err)
	}

	// If the provider is both registered and approved, authorization succeeds
	if regStatus.IsRegistered && regStatus.IsApproved {
		return nil
	}

	// Build contextual error based on status
	var message string
	var publicMessage string

	if !regStatus.IsRegistered {
		message = "provider is not registered with the contract"
		publicMessage = "Provider is not registered. Please run: piri client pdp provider register --name <name> --description <description>"
	} else {
		// Registered but not approved
		message = "provider is registered but not approved by the Storacha team"
		publicMessage = fmt.Sprintf("Provider is registered but awaiting approval from the Storacha team. Please share your Provider ID: %d with the Storacha team to request approval.", regStatus.ID)
	}

	return middleware.NewError(
		"ProviderAuthorization",
		message,
		nil,
		http.StatusForbidden,
	).WithContext("provider_address", p.address.Hex()).
		WithContext("provider_id", regStatus.ID).
		WithContext("is_registered", regStatus.IsRegistered).
		WithContext("is_approved", regStatus.IsApproved).
		WithPublicMessage(publicMessage)
}
