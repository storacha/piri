package tasks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"
	"golang.org/x/xerrors"
	"gorm.io/gorm"

	chainyypes "github.com/filecoin-project/lotus/chain/types"

	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
	"github.com/storacha/piri/pkg/pdp/smartcontracts/bindings"
)

// NewWatcherProviderRegister sets up the watcher for provider registrations
func NewWatcherProviderRegister(db *gorm.DB, pcs *chainsched.Scheduler) error {
	if err := pcs.AddHandler(func(ctx context.Context, revert, apply *chainyypes.TipSet) error {
		err := processPendingProviderRegistrations(ctx, db)
		if err != nil {
			log.Errorf("Failed to process pending provider registrations: %v", err)
		}

		return nil
	}); err != nil {
		return err
	}
	return nil
}

// processPendingProviderRegistrations processes provider registrations that have been confirmed on-chain
func processPendingProviderRegistrations(ctx context.Context, db *gorm.DB) error {
	// Query for pdp_provider_registrations entries where ok = TRUE and provider_registered = FALSE
	var providerRegistrations []models.PDPProviderRegistration
	err := db.WithContext(ctx).
		Where("ok = ? AND provider_registered = ?", true, false).
		Find(&providerRegistrations).Error
	if err != nil {
		return fmt.Errorf("failed to select provider registrations: %w", err)
	}

	if len(providerRegistrations) == 0 {
		// No pending provider registrations
		return nil
	}

	// Process each provider registration
	for _, providerReg := range providerRegistrations {
		err := processProviderRegistration(ctx, db, providerReg)
		if err != nil {
			log.Warnf("Failed to process provider registration for tx %s: %v", providerReg.RegisterMessageHash, err)
			continue
		}
	}

	return nil
}

func processProviderRegistration(ctx context.Context, db *gorm.DB, providerReg models.PDPProviderRegistration) error {
	// Retrieve the tx_receipt from message_waits_eth
	var msgWait models.MessageWaitsEth
	err := db.WithContext(ctx).
		Select("tx_receipt").
		Where("signed_tx_hash = ?", providerReg.RegisterMessageHash).
		First(&msgWait).Error
	if err != nil {
		return fmt.Errorf("failed to get tx_receipt for tx %s: %w", providerReg.RegisterMessageHash, err)
	}
	txReceiptJSON := msgWait.TxReceipt

	// Unmarshal the tx_receipt JSON into types.Receipt
	var txReceipt types.Receipt
	err = json.Unmarshal(txReceiptJSON, &txReceipt)
	if err != nil {
		return xerrors.Errorf("failed to unmarshal tx_receipt for tx %s: %w", providerReg.RegisterMessageHash, err)
	}

	// Parse the logs to extract provider ID
	providerID, err := extractProviderIDFromReceipt(&txReceipt)
	if err != nil {
		return xerrors.Errorf("failed to extract provider ID from receipt for tx %s: %w", providerReg.RegisterMessageHash, err)
	}

	// Update the database with provider ID
	if err := updateProviderRegistration(ctx, db, providerReg.RegisterMessageHash, providerID); err != nil {
		return xerrors.Errorf("failed to update provider registration for tx %s: %w", providerReg.RegisterMessageHash, err)
	}

	return nil
}

func extractProviderIDFromReceipt(receipt *types.Receipt) (uint64, error) {
	// Parse the ServiceProviderRegistry contract ABI
	contractABI, err := bindings.ServiceProviderRegistryMetaData.GetAbi()
	if err != nil {
		return 0, fmt.Errorf("failed to get ServiceProviderRegistry ABI: %w", err)
	}

	// Look for the ProviderRegistered event
	// event ProviderRegistered(uint256 indexed providerId, address indexed providerAddress, address payee);
	for _, vLog := range receipt.Logs {
		if vLog.Address != smartcontracts.Addresses().ProviderRegistry {
			continue
		}

		event, err := contractABI.EventByID(vLog.Topics[0])
		if err != nil {
			continue
		}

		if event.Name == "ProviderRegistered" {
			// The providerId is the first indexed parameter (topics[1])
			if len(vLog.Topics) < 2 {
				continue
			}
			providerID := vLog.Topics[1].Big().Uint64()
			return providerID, nil
		}
	}

	return 0, fmt.Errorf("ProviderRegistered event not found in receipt")
}

func updateProviderRegistration(
	ctx context.Context,
	db *gorm.DB,
	registerMessageHash string,
	providerID uint64,
) error {
	// Begin a database transaction
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Update pdp_provider_registrations to set provider_registered = TRUE and set provider_id
		err := tx.Model(&models.PDPProviderRegistration{}).
			Where("register_message_hash = ?", registerMessageHash).
			Updates(map[string]interface{}{
				"provider_registered": true,
				"provider_id":         int64(providerID),
			}).Error
		if err != nil {
			return fmt.Errorf("failed to update pdp_provider_registrations: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to process provider registration in DB: %w", err)
	}

	return nil
}
