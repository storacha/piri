package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/types"
)

func (p *PDPService) GetProviderStatus(ctx context.Context) (types.GetProviderStatusResults, error) {
	// Check if provider is registered on-chain
	isRegistered, err := p.registryContract.IsRegisteredProvider(ctx, p.address)
	if err != nil {
		return types.GetProviderStatusResults{}, fmt.Errorf("failed to check if service provider is registered: %w", err)
	}

	result := types.GetProviderStatusResults{
		Address: p.address,
	}

	if isRegistered {
		// Provider is registered, get full info
		providerInfoView, err := p.registryContract.GetProviderByAddress(ctx, p.address)
		if err != nil {
			return types.GetProviderStatusResults{}, fmt.Errorf("failed to get provider by address: %w", err)
		}
		approved, err := p.serviceContract.IsProviderApproved(ctx, providerInfoView.ID)
		if err != nil {
			return types.GetProviderStatusResults{}, fmt.Errorf("failed to check if provider is approved: %w", err)
		}
		status := "registered"
		if approved {
			status = "approved"
		}

		result.ID = providerInfoView.ID.Uint64()
		result.Payee = providerInfoView.Payee
		result.IsRegistered = isRegistered
		result.IsActive = providerInfoView.IsActive
		result.Name = providerInfoView.Name
		result.Description = providerInfoView.Description
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
	result.RegistrationStatus = "not_registered"
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

	return fmt.Errorf("provider is not approved")
}
