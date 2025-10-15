package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"github.com/storacha/filecoin-services/go/bindings"
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
		result.ID = providerInfoView.ProviderId.Uint64()
		result.Payee = providerInfoView.Info.Payee
		result.IsRegistered = true
		result.IsActive = providerInfoView.Info.IsActive
		result.Name = providerInfoView.Info.Name
		result.Description = providerInfoView.Info.Description
		result.RegistrationStatus = "registered"
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
