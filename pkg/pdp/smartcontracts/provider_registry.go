package smartcontracts

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/storacha/filecoin-services/go/bindings"
)

type Registry interface {
	IsRegisteredProvider(ctx context.Context, provider common.Address) (bool, error)
	GetProviderByAddress(ctx context.Context, provider common.Address) (*ProviderInfo, error)
	EncodePDPOffering(ctx context.Context, pdpOffering ServiceProviderRegistryStoragePDPOffering) ([]byte, error)
}

type ServiceProviderRegistryStoragePDPOffering struct {
	ServiceURL                 string
	MinPieceSizeInBytes        *big.Int
	MaxPieceSizeInBytes        *big.Int
	IpniPiece                  bool
	IpniIpfs                   bool
	StoragePricePerTibPerMonth *big.Int
	MinProvingPeriodInEpochs   *big.Int
	Location                   string
	PaymentTokenAddress        common.Address
}

type ProviderInfo struct {
	ID              *big.Int
	ServiceProvider common.Address
	Payee           common.Address
	Name            string
	Description     string
	IsActive        bool
}

type serviceProviderRegistry struct {
	registryContract *bindings.ServiceProviderRegistry
	client           bind.ContractBackend
}

func NewRegistry(client bind.ContractBackend) (Registry, error) {
	registryContract, err := bindings.NewServiceProviderRegistry(Addresses().ProviderRegistry, client)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize registry contract: %w", err)
	}
	return &serviceProviderRegistry{
		registryContract: registryContract,
		client:           client,
	}, nil
}

func (r *serviceProviderRegistry) IsRegisteredProvider(ctx context.Context, provider common.Address) (bool, error) {
	return r.registryContract.IsRegisteredProvider(&bind.CallOpts{Context: ctx}, provider)
}

func (r *serviceProviderRegistry) GetProviderByAddress(ctx context.Context, provider common.Address) (*ProviderInfo, error) {
	providerInfo, err := r.registryContract.GetProviderByAddress(&bind.CallOpts{Context: ctx}, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider by address: %w", err)
	}

	return &ProviderInfo{
		ID:          providerInfo.ProviderId,
		Payee:       providerInfo.Info.Payee,
		Name:        providerInfo.Info.Name,
		Description: providerInfo.Info.Description,
		IsActive:    providerInfo.Info.IsActive,
	}, nil
}

func (r *serviceProviderRegistry) EncodePDPOffering(ctx context.Context, pdpOffering ServiceProviderRegistryStoragePDPOffering) ([]byte, error) {
	panic("needs https://github.com/storacha/piri/pull/347")
	// return r.registryContract.EncodePDPOffering(&bind.CallOpts{Context: ctx}, bindings.ServiceProviderRegistryStoragePDPOffering{
	// 	ServiceURL:                 pdpOffering.ServiceURL,
	// 	MinPieceSizeInBytes:        pdpOffering.MinPieceSizeInBytes,
	// 	MaxPieceSizeInBytes:        pdpOffering.MaxPieceSizeInBytes,
	// 	IpniPiece:                  pdpOffering.IpniPiece,
	// 	IpniIpfs:                   pdpOffering.IpniIpfs,
	// 	StoragePricePerTibPerMonth: pdpOffering.StoragePricePerTibPerMonth,
	// 	MinProvingPeriodInEpochs:   pdpOffering.MinProvingPeriodInEpochs,
	// 	Location:                   pdpOffering.Location,
	// 	PaymentTokenAddress:        pdpOffering.PaymentTokenAddress,
	// })
}
