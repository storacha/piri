package service

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
	"github.com/storacha/piri/pkg/pdp/smartcontracts/bindings"
)

const (
	ProductTypePDP uint8 = 0
	// TODO we need to generate type for this from the contract ABI
	// this is based on the contract code, right now there is only a single product type
	// as an enum, so it's value is 0
)

type RegisterProviderParams struct {
	Name        string
	Description string
}

func (p *PDPService) RegisterProvider(ctx context.Context, params RegisterProviderParams) (common.Hash, error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)
		}
	}()
	bindCtx := &bind.CallOpts{Context: ctx}
	registry, err := bindings.NewServiceProviderRegistry(common.HexToAddress(smartcontracts.
		ServiceProviderRegistryProxyAddress), p.ethClient)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to create service registry binding: %w", err)
	}

	isRegistered, err := registry.IsRegisteredProvider(bindCtx, p.address)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to check if service provider is registered: %w", err)
	}

	if isRegistered {
		// TODO we can move this to a separate method for query provider, doing this here because its easy and I lazy.
		providerID, err := registry.GetProviderIdByAddress(bindCtx, p.address)
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to get provider ID by address for registered provider: %w", err)
		}
		providerInfo, err := registry.GetProvider(bindCtx, providerID)
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to get provider info for registered provider: %w", err)
		}
		return common.Hash{}, fmt.Errorf("service provider %d:%s is already registered",
			providerInfo.ProviderId.Uint64(), p.address.Hex())
	}

	// not registered, lets do this
	abiData, err := bindings.ServiceProviderRegistryMetaData.GetAbi()
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get ABI: %w", err)
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
		// so I don't think any of these fields matter, see above comment by claude
		ServiceURL:                 "http://example.com",
		MinPieceSizeInBytes:        big.NewInt(1),
		MaxPieceSizeInBytes:        big.NewInt(2),
		IpniPiece:                  false,
		IpniIpfs:                   false,
		StoragePricePerTibPerMonth: big.NewInt(2),
		MinProvingPeriodInEpochs:   big.NewInt(30),
		Location:                   "earth",
		// TODO validate this, I don't think it's used, but need to verify
		PaymentTokenAddress: p.address,
	})
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to encode product data: %w", err)
	}

	data, err := abiData.Pack("registerProvider", p.address, params.Name, params.Description, ProductTypePDP,
		productData, []string{}, []string{})
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to pack register message abi: %w", err)
	}

	tx := ethtypes.NewTransaction(
		0,
		common.HexToAddress(smartcontracts.ServiceProviderRegistryProxyAddress),
		smartcontracts.RegisterProviderFee(),
		0,
		nil,
		data,
	)

	reason := "register_provider"
	txHash, err := p.sender.Send(ctx, p.address, tx, reason)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to send transaction: %w", err)
	}

	// TODO now we probably need to wait on the message so that we may extract the providerID we get back from this
	// message
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
		return common.Hash{}, err
	}

	return txHash, nil
}
