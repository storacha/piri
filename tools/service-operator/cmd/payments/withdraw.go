package payments

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"

	"github.com/storacha/piri/pkg/pdp/smartcontracts/bindings"
	"github.com/storacha/piri/tools/service-operator/eip712"
	"github.com/storacha/piri/tools/service-operator/internal/contract"
	paymentsutil "github.com/storacha/piri/tools/service-operator/internal/payments"
)

var (
	withdrawAmount string
	withdrawTo     string
)

var withdrawCmd = &cobra.Command{
	Use:   "withdraw",
	Short: "Withdraw funds from the Payments contract",
	Long: `Withdraw available funds from the Payments contract to your wallet or another address.

Only funds that are not locked in active payment rails can be withdrawn.
This is typically used by storage providers to withdraw their settlement earnings.

Examples:
  # Withdraw to own address using keystore
  service-operator payments withdraw --amount 1329414936966 \
    --keystore ./my-keystore --keystore-password password

  # Withdraw to own address using raw private key
  service-operator payments withdraw --amount 1329414936966 \
    --private-key ./wallet-key.hex

  # Withdraw to a specific address
  service-operator payments withdraw --amount 1329414936966 \
    --to 0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb0 \
    --keystore ./my-keystore --keystore-password password`,
	RunE: runWithdraw,
}

func init() {
	withdrawCmd.Flags().StringVar(&withdrawAmount, "amount", "", "Amount to withdraw in base units (required)")
	withdrawCmd.MarkFlagRequired("amount")
	withdrawCmd.Flags().StringVar(&withdrawTo, "to", "", "Address to withdraw to (defaults to your own address)")
}

func runWithdraw(cobraCmd *cobra.Command, args []string) error {
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

	// Validate base configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	// Parse amount
	amount := new(big.Int)
	if _, ok := amount.SetString(withdrawAmount, 10); !ok {
		return fmt.Errorf("invalid amount: %s", withdrawAmount)
	}

	if amount.Sign() <= 0 {
		return fmt.Errorf("amount must be greater than zero")
	}

	// Validate 'to' address if provided
	var toAddr common.Address
	if withdrawTo != "" {
		if !common.IsHexAddress(withdrawTo) {
			return fmt.Errorf("invalid 'to' address: %s", withdrawTo)
		}
		toAddr = common.HexToAddress(withdrawTo)
	}

	client, err := ethclient.Dial(cfg.RPCUrl)
	if err != nil {
		return fmt.Errorf("connecting to RPC endpoint: %w", err)
	}
	defer client.Close()

	// Load private key for signing
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

	fromAddr := crypto.PubkeyToAddress(privateKey.PublicKey)

	// If no 'to' address specified, use fromAddr
	if withdrawTo == "" {
		toAddr = fromAddr
	}

	// Get chain ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("getting chain ID: %w", err)
	}

	// Query token decimals for display
	decimals, err := eip712.GetTokenDecimals(ctx, client, cfg.TokenAddr())
	if err != nil {
		return fmt.Errorf("querying token decimals: %w", err)
	}

	// Query current account balance to validate
	paymentsContract, err := bindings.NewPayments(cfg.PaymentsAddr(), client)
	if err != nil {
		return fmt.Errorf("creating payments contract binding: %w", err)
	}

	accountInfo, err := paymentsContract.Accounts(nil, cfg.TokenAddr(), fromAddr)
	if err != nil {
		return fmt.Errorf("querying account information: %w", err)
	}

	availableFunds := new(big.Int).Sub(accountInfo.Funds, accountInfo.LockupCurrent)

	// Validate amount doesn't exceed available
	if amount.Cmp(availableFunds) > 0 {
		return fmt.Errorf("insufficient available funds: requested %s (%s), available %s (%s)",
			amount.String(),
			paymentsutil.FormatTokenAmount(amount, decimals),
			availableFunds.String(),
			paymentsutil.FormatTokenAmount(availableFunds, decimals))
	}

	// Display withdrawal info
	fmt.Println("Withdraw from Payments Contract")
	fmt.Println("================================")
	fmt.Println()
	fmt.Printf("From address:       %s\n", fromAddr.Hex())
	fmt.Printf("To address:         %s\n", toAddr.Hex())
	fmt.Printf("Amount:             %s (%s)\n",
		amount.String(),
		paymentsutil.FormatTokenAmount(amount, decimals))
	fmt.Printf("Available balance:  %s (%s)\n",
		availableFunds.String(),
		paymentsutil.FormatTokenAmount(availableFunds, decimals))
	fmt.Println()

	// Create transaction auth
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		return fmt.Errorf("creating transaction auth: %w", err)
	}

	// Create transactor
	paymentsTransactor, err := bindings.NewPaymentsTransactor(cfg.PaymentsAddr(), client)
	if err != nil {
		return fmt.Errorf("creating payments transactor: %w", err)
	}

	// Execute withdrawal
	fmt.Println("Sending withdrawal transaction...")

	var tx *types.Transaction

	if withdrawTo != "" && toAddr != fromAddr {
		// Use withdrawTo
		tx, err = paymentsTransactor.WithdrawTo(auth, cfg.TokenAddr(), toAddr, amount)
		if err != nil {
			return fmt.Errorf("withdrawal failed: %w", err)
		}
	} else {
		// Use withdraw (to self)
		tx, err = paymentsTransactor.Withdraw(auth, cfg.TokenAddr(), amount)
		if err != nil {
			return fmt.Errorf("withdrawal failed: %w", err)
		}
	}

	fmt.Printf("Transaction submitted: %s\n", tx.Hash().Hex())
	fmt.Println("Waiting for confirmation...")

	// Wait for transaction to be mined
	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		return fmt.Errorf("waiting for transaction to be mined: %w", err)
	}

	if receipt.Status != 1 {
		return fmt.Errorf("transaction failed with status %d", receipt.Status)
	}

	// Display success
	fmt.Println()
	fmt.Println("✓ Withdrawal successful!")
	fmt.Printf("Transaction hash:   %s\n", tx.Hash().Hex())
	fmt.Printf("Withdrawn:          %s (%s)\n",
		amount.String(),
		paymentsutil.FormatTokenAmount(amount, decimals))
	fmt.Println()

	// Calculate and show remaining balance
	remainingAvailable := new(big.Int).Sub(availableFunds, amount)
	fmt.Printf("Remaining available balance: %s (%s)\n",
		remainingAvailable.String(),
		paymentsutil.FormatTokenAmount(remainingAvailable, decimals))

	return nil
}
