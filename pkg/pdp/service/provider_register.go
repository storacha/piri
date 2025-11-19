package service

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/storacha/piri/pkg/pdp/types"
	"gorm.io/gorm"

	"github.com/storacha/filecoin-services/go/bindings"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
)

func (p *PDPService) RegisterProvider(ctx context.Context, params types.RegisterProviderParams) (types.RegisterProviderResults, error) {
	// Check for pending registration in database
	var pendingReg models.PDPProviderRegistration
	err := p.db.WithContext(ctx).
		Where("service = ? AND provider_registered = ?", p.name, false).
		First(&pendingReg).Error

	if err == nil {
		// Found a pending registration
		return types.RegisterProviderResults{}, types.NewError(types.KindConflict, fmt.Sprintf("provider registration already in progress (tx: %s)", pendingReg.RegisterMessageHash))
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return types.RegisterProviderResults{}, fmt.Errorf("failed to check for pending registration: %w", err)
	}

	isRegistered, err := p.registryContract.IsRegisteredProvider(ctx, p.address)
	if err != nil {
		return types.RegisterProviderResults{}, fmt.Errorf("failed to check if service provider is registered: %w", err)
	}

	if isRegistered {
		return types.RegisterProviderResults{}, types.NewError(types.KindConflict, "Provider is already registered")
	}

	// not registered, lets do this
	abiData, err := bindings.ServiceProviderRegistryMetaData.GetAbi()
	if err != nil {
		return types.RegisterProviderResults{}, fmt.Errorf("failed to get ABI: %w", err)
	}

	productData, err := p.registryContract.EncodePDPOffering(ctx, smartcontracts.ServiceProviderRegistryStoragePDPOffering{
		// None of these fields except PaymentTokenAddress are used by the service contract, they simply serve as an
		// unused on-chain registy of data.
		// TODO: later, we way want to allow node providers to pick these themselves, unsure what value that adds currently
		// but this does represent information that are advertising on chain.
		ServiceURL:                 "https://storacha.network",
		MinPieceSizeInBytes:        big.NewInt(1),
		MaxPieceSizeInBytes:        big.NewInt(1),
		IpniPiece:                  false,
		IpniIpfs:                   false,
		StoragePricePerTibPerMonth: big.NewInt(1),
		MinProvingPeriodInEpochs:   big.NewInt(1),
		Location:                   "earth",
		// This field DOES matter as it's the address payment will be issued to by the contract.
		PaymentTokenAddress: p.address,
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
		p.cfg.Contracts.ProviderRegistry,
		smartcontracts.RegisterFee,
		0,
		nil,
		data,
	)

	reason := "register_provider"
	txHash, err := p.sender.Send(ctx, p.address, tx, reason)
	if err != nil {
		return types.RegisterProviderResults{}, fmt.Errorf("failed to send transaction: %w", err)
	}

	if err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		msgWait := models.MessageWaitsEth{
			SignedTxHash: txHash.Hex(),
			TxStatus:     "pending",
		}
		if err := tx.Create(&msgWait).Error; err != nil {
			return fmt.Errorf("failed to insert into %s: %w", msgWait.TableName(), err)
		}

		// Insert into pdp_provider_registrations
		providerReg := models.PDPProviderRegistration{
			RegisterMessageHash: txHash.Hex(),
			Service:             p.name,
			ProviderRegistered:  false,
		}
		if err := tx.Create(&providerReg).Error; err != nil {
			return fmt.Errorf("failed to insert into %s: %w", providerReg.TableName(), err)
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
