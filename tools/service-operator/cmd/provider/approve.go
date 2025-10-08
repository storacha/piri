package provider

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"

	"github.com/storacha/piri/pkg/pdp/smartcontracts/bindings"
	"github.com/storacha/piri/tools/service-operator/internal/contract"
)

var approveCmd = &cobra.Command{
	Use:   "approve <provider-id>",
	Short: "Approve a provider to create datasets",
	Long: `Approve a provider by their ID to allow them to create datasets in the FilecoinWarmStorageService.

The provider must already be registered in the ServiceProviderRegistry before approval.
Only the contract owner can approve providers.`,
	Args: cobra.ExactArgs(1),
	RunE: runApprove,
}

// TODO: room for improvement here, this method will return success even if:
// 1. the provider is already approved
// 2. the provider doesn't exist
func runApprove(cobraCmd *cobra.Command, args []string) error {
	ctx := cobraCmd.Context()

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	providerID := new(big.Int)
	if _, ok := providerID.SetString(args[0], 10); !ok {
		return fmt.Errorf("invalid provider ID: %s (must be a valid number)", args[0])
	}

	fmt.Printf("Approving provider ID: %s\n", providerID.String())
	fmt.Printf("Contract: %s\n", cfg.ContractAddress)
	fmt.Printf("RPC URL: %s\n", cfg.RPCUrl)
	fmt.Println()

	client, err := ethclient.Dial(cfg.RPCUrl)
	if err != nil {
		return fmt.Errorf("connecting to RPC endpoint: %w", err)
	}
	defer client.Close()

	var privateKey *ecdsa.PrivateKey
	if cfg.PrivateKeyPath != "" {
		privateKey, err = contract.LoadPrivateKey(cfg.PrivateKeyPath)
		if err != nil {
			return fmt.Errorf("loading private key: %w", err)
		}
	} else {
		privateKey, err = contract.LoadPrivateKeyFromKeystore(cfg.KeystorePath, cfg.KeystorePassword)
		if err != nil {
			return fmt.Errorf("loading keystore: %w", err)
		}
	}

	auth, err := contract.CreateTransactor(ctx, client, privateKey)
	if err != nil {
		return fmt.Errorf("creating transactor: %w", err)
	}

	contractInstance, err := bindings.NewFilecoinWarmStorageService(cfg.ContractAddr(), client)
	if err != nil {
		return fmt.Errorf("creating contract binding: %w", err)
	}

	tx, err := contractInstance.AddApprovedProvider(auth, providerID)
	if err != nil {
		return fmt.Errorf("calling AddApprovedProvider: %w", err)
	}

	receipt, err := contract.WaitForTransaction(ctx, client, tx.Hash())
	if err != nil {
		return fmt.Errorf("waiting for transaction: %w", err)
	}

	approvedID, err := contract.GetProviderApprovedEvent(receipt)
	if err != nil {
		return fmt.Errorf("parsing event: %w", err)
	}

	fmt.Println()
	fmt.Printf("✓ Provider %s approved successfully!\n", approvedID.String())
	fmt.Printf("Transaction: %s\n", receipt.TxHash.Hex())
	fmt.Printf("Block: %d\n", receipt.BlockNumber.Uint64())
	fmt.Printf("Gas used: %d\n", receipt.GasUsed)

	return nil
}
