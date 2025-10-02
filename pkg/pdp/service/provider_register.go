package service

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/storacha/piri/pkg/pdp/types"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
	"github.com/storacha/piri/pkg/pdp/smartcontracts/bindings"
)

func (p *PDPService) RegisterProvider(ctx context.Context, params types.RegisterProviderParams) (types.RegisterProviderResults, error) {
	// TODO(forrest): remove this once confident in the code
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in RegisterProvider", r)
		}
	}()
	bindCtx := &bind.CallOpts{Context: ctx}
	registry, err := bindings.NewServiceProviderRegistry(smartcontracts.Addresses().ProviderRegistry, p.contractBackend)
	if err != nil {
		return types.RegisterProviderResults{}, fmt.Errorf("failed to create service registry binding: %w", err)
	}

	isRegistered, err := registry.IsRegisteredProvider(bindCtx, p.address)
	if err != nil {
		return types.RegisterProviderResults{}, fmt.Errorf("failed to check if service provider is registered: %w", err)
	}

	if isRegistered {
		// TODO we can move this to a separate method for query provider, doing this here because its easy and I lazy.
		providerInfoView, err := registry.GetProviderByAddress(bindCtx, p.address)
		if err != nil {
			return types.RegisterProviderResults{}, fmt.Errorf("failed to get provider by address for registered provider: %w", err)
		}
		log.Errorf("service provider %s is already registered with address %s", providerInfoView.ProviderId, p.address)
		return types.RegisterProviderResults{
			Address:     providerInfoView.Info.ServiceProvider,
			Payee:       providerInfoView.Info.Payee,
			ID:          providerInfoView.ProviderId.Uint64(),
			IsActive:    providerInfoView.Info.IsActive,
			Name:        providerInfoView.Info.Name,
			Description: providerInfoView.Info.Description,
		}, nil
	}

	// not registered, lets do this
	abiData, err := bindings.ServiceProviderRegistryMetaData.GetAbi()
	if err != nil {
		return types.RegisterProviderResults{}, fmt.Errorf("failed to get ABI: %w", err)
	}

	/*
	 /// @notice Register as a new service provider with a specific product type
	    /// @param payee Address that will receive payments (cannot be changed after registration)
	    /// @param name Provider name (optional, max 128 chars)
	    /// @param description Provider description (max 256 chars)
	    /// @param productType The type of product to register
	    /// @param productData The encoded product configuration data
	    /// @param capabilityKeys Array of capability keys
	    /// @param capabilityValues Array of capability values
	    /// @return providerId The unique ID assigned to the provider
	    function registerProvider(
	        address payee,
	        string calldata name,
	        string calldata description,
	        ProductType productType,
	        bytes calldata productData,
	        string[] calldata capabilityKeys,
	        string[] calldata capabilityValues
	    ) external payable returns (uint256 providerId) {
	*/

	/*
		The PDPOffering data (serviceURL, minPieceSizeInBytes,
		  maxPieceSizeInBytes, storagePricePerTibPerMonth, etc.) is completely
		  ignored.
		  Instead, FilecoinWarmStorageService uses its own:
		  - Fixed pricing: 5 USDFC per TiB/month (line 302)
		  - Fixed proving periods: Set during initialization (lines 345-346)
		  - No piece size restrictions from PDPOffering
		  So, the PDPOffering is just stored in the
		  registry for discovery/informational purposes. It's not used
		  operationally by the FilecoinWarmStorageService contract.
		  This makes sense architecturally because:
		  1. The registry acts as a "yellow pages" where providers advertise their
		  capabilities
		  2. The actual service contract (FilecoinWarmStorageService) enforces its
		  own standardized terms
		  3. Clients might use PDPOffering data to discover providers, but the
		  actual service operates on fixed terms
	*/
	productData, err := registry.EncodePDPOffering(bindCtx, bindings.ServiceProviderRegistryStoragePDPOffering{
		// so I don't think any of these fields matter, see above comment
		// TODO validate this assumption, I don't think it's used, but need to verify
		ServiceURL:                 "http://example.com",
		MinPieceSizeInBytes:        big.NewInt(1),
		MaxPieceSizeInBytes:        big.NewInt(2),
		IpniPiece:                  false,
		IpniIpfs:                   false,
		StoragePricePerTibPerMonth: big.NewInt(2),
		MinProvingPeriodInEpochs:   big.NewInt(30),
		Location:                   "earth",
		PaymentTokenAddress:        p.address,
	})
	if err != nil {
		return types.RegisterProviderResults{}, fmt.Errorf("failed to encode product data: %w", err)
	}

	data, err := abiData.Pack("registerProvider", p.address, params.Name, params.Description, types.ProductTypePDP,
		productData, []string{}, []string{})
	if err != nil {
		return types.RegisterProviderResults{}, fmt.Errorf("failed to pack register message abi: %w", err)
	}

	tx := ethtypes.NewTransaction(
		0,
		smartcontracts.Addresses().ProviderRegistry,
		smartcontracts.RegisterProviderFee(),
		0,
		nil,
		data,
	)

	reason := "register_provider"
	txHash, err := p.sender.Send(ctx, p.address, tx, reason)
	if err != nil {
		return types.RegisterProviderResults{}, fmt.Errorf("failed to send transaction: %w", err)
	}

	// NB(forrest): we could define a new database model and task, e.g. watch_providerregister.go that listens
	// for successful messages, parses the receipts, and stores the provider ID in the database.
	// But that's a lot of work for something that will really only happen once in a providers' lifetime.
	// so instead, at the top of this function we just check if the provider is registered, and if they are return the
	// providerID, allowing this method to be called repeatedly until an ID is returned, which is lazy...so
	// TODO: evaluate this comment, and complete it or delete the comment and TODO
	if err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		msgWait := models.MessageWaitsEth{
			SignedTxHash: txHash.Hex(),
			TxStatus:     "pending",
		}
		if err := tx.Create(&msgWait).Error; err != nil {
			return fmt.Errorf("failed to insert into %s: %w", msgWait.TableName(), err)
		}

		// Return nil to commit the transaction.
		return nil
	}); err != nil {
		return types.RegisterProviderResults{}, err
	}

	return types.RegisterProviderResults{
		TransactionHash: txHash,
	}, nil
}
