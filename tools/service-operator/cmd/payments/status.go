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
	"github.com/storacha/piri/tools/service-operator/internal/payments"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display account balance and operator approval status",
	Long: `Display your current account balance in the Payments contract and the
approval status of the FilecoinWarmStorageService contract as an operator.

This shows:
- Your account balance (funds and lockup information)
- Operator approval status (allowances and usage)
- Available capacity for creating new payment rails

Examples:
  # Check status on calibration network
  service-operator payments status --network calibration

  # Check status with explicit addresses
  service-operator payments status \
    --rpc-url https://api.calibration.node.glif.io/rpc/v1 \
    --payments-address 0x6dB198201F900c17e86D267d7Df82567FB03df5E \
    --token-address 0xb3042734b608a1B16e9e86B374A3f3e389B4cDf0 \
    --contract-address 0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91 \
    --private-key ./wallet-key.hex`,
	RunE: runStatus,
}

func runStatus(cobraCmd *cobra.Command, args []string) error {
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

	if cfg.ContractAddress == "" {
		return fmt.Errorf("contract-address is required")
	}
	if !common.IsHexAddress(cfg.ContractAddress) {
		return fmt.Errorf("invalid service contract address: %s", cfg.ContractAddress)
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

	paymentsContract, err := bindings.NewPayments(cfg.PaymentsAddr(), client)
	if err != nil {
		return fmt.Errorf("creating payments contract binding: %w", err)
	}

	// Query account information
	accountInfo, err := paymentsContract.Accounts(nil, cfg.TokenAddr(), ownerAddr)
	if err != nil {
		return fmt.Errorf("querying account information: %w", err)
	}

	// Query operator approval information
	operatorInfo, err := paymentsContract.OperatorApprovals(nil, cfg.TokenAddr(), ownerAddr, cfg.ContractAddr())
	if err != nil {
		return fmt.Errorf("querying operator approval: %w", err)
	}

	// Display results
	fmt.Println("Payments Account Status")
	fmt.Println("=======================")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Printf("  Payments contract:      %s\n", cfg.PaymentsContractAddress)
	fmt.Printf("  Token contract:         %s\n", cfg.TokenContractAddress)
	fmt.Printf("  Service contract:       %s\n", cfg.ContractAddress)
	fmt.Printf("  Your address:           %s\n", ownerAddr.Hex())
	fmt.Printf("  RPC URL:                %s\n", cfg.RPCUrl)
	fmt.Println()

	fmt.Println("Account Balance:")
	fmt.Printf("  Total funds:            %s (%s)\n",
		accountInfo.Funds.String(),
		payments.FormatTokenAmount(accountInfo.Funds, decimals))
	fmt.Printf("  Locked funds:           %s (%s)\n",
		accountInfo.LockupCurrent.String(),
		payments.FormatTokenAmount(accountInfo.LockupCurrent, decimals))

	// Calculate available funds
	availableFunds := new(big.Int).Sub(accountInfo.Funds, accountInfo.LockupCurrent)
	fmt.Printf("  Available funds:        %s (%s)\n",
		availableFunds.String(),
		payments.FormatTokenAmount(availableFunds, decimals))
	fmt.Println()

	fmt.Println("Operator Approval Status:")
	if !operatorInfo.IsApproved {
		fmt.Println("  Status:                 ❌ Not approved")
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  1. Calculate allowances: service-operator payments calculate --size <dataset-size>")
		fmt.Println("  2. Approve operator: service-operator payments approve-service \\")
		fmt.Println("       --rate-allowance <value> \\")
		fmt.Println("       --lockup-allowance <value> \\")
		fmt.Println("       --max-lockup-period <value>")
	} else {
		fmt.Println("  Status:                 ✓ Approved")
		fmt.Println()
		fmt.Println("  Rate Allowance:")
		fmt.Printf("    Total allowance:      %s/epoch (%s/epoch)\n",
			operatorInfo.RateAllowance.String(),
			payments.FormatTokenAmount(operatorInfo.RateAllowance, payments.TokenDecimals))
		fmt.Printf("    Currently used:       %s/epoch (%s/epoch)\n",
			operatorInfo.RateUsage.String(),
			payments.FormatTokenAmount(operatorInfo.RateUsage, payments.TokenDecimals))
		rateAvailable := new(big.Int).Sub(operatorInfo.RateAllowance, operatorInfo.RateUsage)
		fmt.Printf("    Available:            %s/epoch (%s/epoch)\n",
			rateAvailable.String(),
			payments.FormatTokenAmount(rateAvailable, payments.TokenDecimals))
		fmt.Println()

		fmt.Println("  Lockup Allowance:")
		fmt.Printf("    Total allowance:      %s (%s)\n",
			operatorInfo.LockupAllowance.String(),
			payments.FormatTokenAmount(operatorInfo.LockupAllowance, payments.TokenDecimals))
		fmt.Printf("    Currently used:       %s (%s)\n",
			operatorInfo.LockupUsage.String(),
			payments.FormatTokenAmount(operatorInfo.LockupUsage, payments.TokenDecimals))
		lockupAvailable := new(big.Int).Sub(operatorInfo.LockupAllowance, operatorInfo.LockupUsage)
		fmt.Printf("    Available:            %s (%s)\n",
			lockupAvailable.String(),
			payments.FormatTokenAmount(lockupAvailable, payments.TokenDecimals))
		fmt.Println()

		fmt.Printf("  Max Lockup Period:      %s epochs (%d days)\n",
			operatorInfo.MaxLockupPeriod.String(),
			operatorInfo.MaxLockupPeriod.Int64()/2880)
	}

	return nil
}