package payments

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"

	"github.com/storacha/piri/tools/service-operator/eip712"
	"github.com/storacha/piri/tools/service-operator/internal/contract"
	paymentsutil "github.com/storacha/piri/tools/service-operator/internal/payments"
)

var balanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Display USDFC token balance in your wallet",
	Long: `Display the USDFC token balance in your wallet (not in the Payments contract).

This shows how much USDFC you have available to deposit into the Payments contract.

To see your balance IN the Payments contract, use 'payments status' instead.

Examples:
  # Check wallet balance on calibration network
  service-operator payments balance --network calibration

  # Check balance with explicit addresses
  service-operator payments balance \
    --rpc-url https://api.calibration.node.glif.io/rpc/v1 \
    --token-address 0xb3042734b608a1B16e9e86B374A3f3e389B4cDf0 \
    --private-key ./wallet-key.hex`,
	RunE: runBalance,
}

func runBalance(cobraCmd *cobra.Command, args []string) error {
	ctx := cobraCmd.Context()

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Validate token configuration
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

	client, err := ethclient.Dial(cfg.RPCUrl)
	if err != nil {
		return fmt.Errorf("connecting to RPC endpoint: %w", err)
	}
	defer client.Close()

	// Load private key to get address
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

	ownerAddr := crypto.PubkeyToAddress(privateKey.PublicKey)

	// Query token decimals
	decimals, err := eip712.GetTokenDecimals(ctx, client, cfg.TokenAddr())
	if err != nil {
		return fmt.Errorf("querying token decimals: %w", err)
	}

	// Query token balance
	balance, err := eip712.GetTokenBalance(ctx, client, cfg.TokenAddr(), ownerAddr)
	if err != nil {
		return fmt.Errorf("querying token balance: %w", err)
	}

	// Display results
	fmt.Println("USDFC Wallet Balance")
	fmt.Println("====================")
	fmt.Println()
	fmt.Printf("Token contract:  %s\n", cfg.TokenContractAddress)
	fmt.Printf("Your address:    %s\n", ownerAddr.Hex())
	fmt.Printf("RPC URL:         %s\n", cfg.RPCUrl)
	fmt.Println()
	fmt.Printf("Wallet balance:  %s base units (%s)\n",
		balance.String(),
		paymentsutil.FormatTokenAmount(balance, decimals))
	fmt.Println()

	if balance.Sign() == 0 {
		fmt.Println("⚠ Your wallet has no USDFC tokens")
		fmt.Println()
		fmt.Println("To use the payments system, you need to:")
		fmt.Println("  1. Acquire USDFC tokens")
		fmt.Println("  2. Deposit them: service-operator payments deposit --amount <amount>")
	} else {
		fmt.Println("You can deposit this balance into the Payments contract:")
		fmt.Printf("  service-operator payments deposit --amount %s\n", balance.String())
	}

	return nil
}
