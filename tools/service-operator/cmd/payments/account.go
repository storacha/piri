package payments

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"

	"github.com/storacha/piri/pkg/pdp/smartcontracts/bindings"
	"github.com/storacha/piri/tools/service-operator/eip712"
	"github.com/storacha/piri/tools/service-operator/internal/contract"
	paymentsutil "github.com/storacha/piri/tools/service-operator/internal/payments"
)

var (
	accountAddress string
)

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Display account balance in the Payments contract",
	Long: `Display the account balance stored in the Payments contract.

This shows the funds that have been deposited or received through settlement,
including total funds, locked funds, and available funds that can be withdrawn.

This is useful for storage providers to check their settlement earnings.

Examples:
  # Check your own account balance using keystore
  service-operator payments account --keystore ./my-keystore

  # Check balance of any address (read-only, no keystore needed)
  service-operator payments account --address 0x7469B47e006D0660aB92AE560b27A1075EEcF97F

  # Check balance on calibration network
  service-operator payments account --network calibration`,
	RunE: runAccount,
}

func init() {
	accountCmd.Flags().StringVar(&accountAddress, "address", "", "Address to check (defaults to keystore address if not specified)")
}

func runAccount(cobraCmd *cobra.Command, args []string) error {
	ctx := cobraCmd.Context()

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Validate payments-specific configuration
	if cfg.PaymentsContractAddress == "" {
		return fmt.Errorf("payments-address is required")
	}
	if !common.IsHexAddress(cfg.PaymentsContractAddress) {
		return fmt.Errorf("invalid payments contract address: %s", cfg.PaymentsContractAddress)
	}

	if cfg.TokenContractAddress == "" {
		return fmt.Errorf("token-address is required")
	}
	if !common.IsHexAddress(cfg.TokenContractAddress) {
		return fmt.Errorf("invalid token contract address: %s", cfg.TokenContractAddress)
	}

	// Validate RPC URL
	if cfg.RPCUrl == "" {
		return fmt.Errorf("rpc-url is required")
	}

	client, err := ethclient.Dial(cfg.RPCUrl)
	if err != nil {
		return fmt.Errorf("connecting to RPC endpoint: %w", err)
	}
	defer client.Close()

	// Determine which address to query
	var queryAddr common.Address
	if accountAddress != "" {
		// User specified an address - validate it
		if !common.IsHexAddress(accountAddress) {
			return fmt.Errorf("invalid address: %s", accountAddress)
		}
		queryAddr = common.HexToAddress(accountAddress)
	} else {
		// No address specified - use keystore address
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
		queryAddr = crypto.PubkeyToAddress(privateKey.PublicKey)
	}

	// Query token decimals
	decimals, err := eip712.GetTokenDecimals(ctx, client, cfg.TokenAddr())
	if err != nil {
		return fmt.Errorf("querying token decimals: %w", err)
	}

	// Query Payments contract for account balance
	paymentsContract, err := bindings.NewPayments(cfg.PaymentsAddr(), client)
	if err != nil {
		return fmt.Errorf("creating payments contract binding: %w", err)
	}

	accountInfo, err := paymentsContract.Accounts(nil, cfg.TokenAddr(), queryAddr)
	if err != nil {
		return fmt.Errorf("querying account information: %w", err)
	}

	// Calculate available funds
	availableFunds := new(big.Int).Sub(accountInfo.Funds, accountInfo.LockupCurrent)

	// Display results
	fmt.Println("Payments Account Balance")
	fmt.Println("========================")
	fmt.Println()
	fmt.Printf("Payments contract:  %s\n", cfg.PaymentsContractAddress)
	fmt.Printf("Token contract:     %s\n", cfg.TokenContractAddress)
	fmt.Printf("Account address:    %s\n", queryAddr.Hex())
	fmt.Printf("RPC URL:            %s\n", cfg.RPCUrl)
	fmt.Println()

	fmt.Println("Balance in Payments Contract:")
	fmt.Printf("  Total funds:      %s (%s)\n",
		accountInfo.Funds.String(),
		paymentsutil.FormatTokenAmount(accountInfo.Funds, decimals))
	fmt.Printf("  Locked funds:     %s (%s)\n",
		accountInfo.LockupCurrent.String(),
		paymentsutil.FormatTokenAmount(accountInfo.LockupCurrent, decimals))
	fmt.Printf("  Available funds:  %s (%s)\n",
		availableFunds.String(),
		paymentsutil.FormatTokenAmount(availableFunds, decimals))
	fmt.Println()

	// Show helpful next steps based on balance
	if accountInfo.Funds.Sign() == 0 {
		fmt.Println("📭 This account has no funds in the Payments contract.")
		fmt.Println()
		fmt.Println("To deposit funds:")
		fmt.Println("  service-operator payments deposit --amount <amount>")
	} else if availableFunds.Sign() > 0 {
		fmt.Println("💰 You have available funds that can be withdrawn.")
		fmt.Println()
		fmt.Printf("To withdraw all available funds:\n")
		fmt.Printf("  service-operator payments withdraw --amount %s\n", availableFunds.String())
		fmt.Println()
		fmt.Printf("To withdraw to a specific address:\n")
		fmt.Printf("  service-operator payments withdraw --amount %s --to <address>\n", availableFunds.String())
	} else if accountInfo.LockupCurrent.Sign() > 0 {
		fmt.Println("🔒 All funds are currently locked in active payment rails.")
		fmt.Println()
		fmt.Println("Locked funds become available after settling payment rails:")
		fmt.Println("  service-operator payments settle --all")
	}

	return nil
}
