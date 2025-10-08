package payments

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

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
	depositAmount   string
	rateAllowance   string
	lockupAllowance string
	maxLockupPeriod uint64
	permitDeadline  uint64
	includeDeposit  bool
)

var approveServiceCmd = &cobra.Command{
	Use:   "approve-service",
	Short: "Approve the FilecoinWarmStorageService contract as an operator",
	Long: `Approve the FilecoinWarmStorageService contract to operate on your behalf in the Payments contract.

This command can either:
1. Approve the service contract as an operator (--deposit=false)
2. Deposit tokens with permit and approve the operator in one transaction (--deposit=true)

The EIP-2612 permit signature is generated internally using your private key.

Examples:
  # Approve service contract as operator (no deposit)
  service-operator payments approve-service \
    --rate-allowance 1000000 \
    --lockup-allowance 5000000 \
    --max-lockup-period 2592000

  # Deposit and approve in one transaction
  service-operator payments approve-service \
    --deposit \
    --amount 10000000 \
    --rate-allowance 1000000 \
    --lockup-allowance 5000000 \
    --max-lockup-period 2592000`,
	RunE: runApproveService,
}

func init() {
	approveServiceCmd.Flags().StringVar(&depositAmount, "amount", "", "Amount to deposit (required if --deposit=true)")
	approveServiceCmd.Flags().StringVar(&rateAllowance, "rate-allowance", "", "Rate allowance for the operator (required)")
	approveServiceCmd.Flags().StringVar(&lockupAllowance, "lockup-allowance", "", "Lockup allowance for the operator (required)")
	approveServiceCmd.Flags().Uint64Var(&maxLockupPeriod, "max-lockup-period", 0, "Maximum lockup period in seconds (required)")
	approveServiceCmd.Flags().Uint64Var(&permitDeadline, "permit-deadline", 0, "Permit deadline as unix timestamp (default: 1 hour from now)")
	approveServiceCmd.Flags().BoolVar(&includeDeposit, "deposit", false, "Include deposit with permit in the same transaction")

	cobra.MarkFlagRequired(approveServiceCmd.Flags(), "rate-allowance")
	cobra.MarkFlagRequired(approveServiceCmd.Flags(), "lockup-allowance")
	cobra.MarkFlagRequired(approveServiceCmd.Flags(), "max-lockup-period")
}

func runApproveService(cobraCmd *cobra.Command, args []string) error {
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

	if includeDeposit {
		if cfg.TokenContractAddress == "" {
			return fmt.Errorf("token-address is required when using --deposit")
		}
		if !common.IsHexAddress(cfg.TokenContractAddress) {
			return fmt.Errorf("invalid token contract address: %s", cfg.TokenContractAddress)
		}
		if depositAmount == "" {
			return fmt.Errorf("--amount is required when using --deposit")
		}
	}

	// Validate base configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	// Parse allowances
	rateAllowanceBig := new(big.Int)
	if _, ok := rateAllowanceBig.SetString(rateAllowance, 10); !ok {
		return fmt.Errorf("invalid rate-allowance: %s (must be a valid number)", rateAllowance)
	}

	lockupAllowanceBig := new(big.Int)
	if _, ok := lockupAllowanceBig.SetString(lockupAllowance, 10); !ok {
		return fmt.Errorf("invalid lockup-allowance: %s (must be a valid number)", lockupAllowance)
	}

	// Parse deposit amount if provided
	var amountBig *big.Int
	if includeDeposit {
		amountBig = new(big.Int)
		if _, ok := amountBig.SetString(depositAmount, 10); !ok {
			return fmt.Errorf("invalid amount: %s (must be a valid number)", depositAmount)
		}
	}

	// Set default permit deadline if not provided
	if permitDeadline == 0 {
		permitDeadline = uint64(time.Now().Add(1 * time.Hour).Unix())
	}

	fmt.Println("Approving FilecoinWarmStorageService contract as operator")
	fmt.Printf("Payments contract: %s\n", cfg.PaymentsContractAddress)
	fmt.Printf("Service contract: %s\n", cfg.ContractAddress)
	if includeDeposit {
		fmt.Printf("Token contract: %s\n", cfg.TokenContractAddress)
		fmt.Printf("Deposit amount: %s\n", amountBig.String())
	}
	fmt.Printf("Rate allowance: %s\n", rateAllowanceBig.String())
	fmt.Printf("Lockup allowance: %s\n", lockupAllowanceBig.String())
	fmt.Printf("Max lockup period: %d seconds\n", maxLockupPeriod)
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

	// Get chain ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("getting chain ID: %w", err)
	}

	paymentsContract, err := bindings.NewPayments(cfg.PaymentsAddr(), client)
	if err != nil {
		return fmt.Errorf("creating payments contract binding: %w", err)
	}

	var tx *types.Transaction
	var receipt *types.Receipt

	// Get owner address from private key (needed for balance check and deposit)
	ownerAddr := crypto.PubkeyToAddress(privateKey.PublicKey)

	if includeDeposit {
		// Check token balance before attempting deposit
		fmt.Println("Checking token balance...")

		decimals, err := eip712.GetTokenDecimals(ctx, client, cfg.TokenAddr())
		if err != nil {
			return fmt.Errorf("querying token decimals: %w", err)
		}

		balance, err := eip712.GetTokenBalance(ctx, client, cfg.TokenAddr(), ownerAddr)
		if err != nil {
			return fmt.Errorf("querying token balance: %w", err)
		}

		fmt.Printf("Current balance:    %s base units (%s)\n", balance.String(), paymentsutil.FormatTokenAmount(balance, decimals))

		if balance.Cmp(amountBig) < 0 {
			return fmt.Errorf("insufficient token balance: have %s, need %s", balance.String(), amountBig.String())
		}
		fmt.Println()

		// Generate EIP-2612 permit signature
		fmt.Println("Generating EIP-2612 permit signature...")
		permitSig, err := eip712.GeneratePermitSignature(
			ctx,
			client,
			cfg.TokenAddr(),
			cfg.PaymentsAddr(), // Spender is the Payments contract
			amountBig,
			big.NewInt(int64(permitDeadline)),
			privateKey,
			chainID,
		)
		if err != nil {
			return fmt.Errorf("generating permit signature: %w", err)
		}

		fmt.Println("Calling depositWithPermitAndApproveOperator...")

		// Call depositWithPermitAndApproveOperator
		tx, err = paymentsContract.DepositWithPermitAndApproveOperator(
			auth,
			cfg.TokenAddr(),
			ownerAddr, // Deposit to the caller's account
			amountBig,
			big.NewInt(int64(permitDeadline)),
			permitSig.V,
			permitSig.R,
			permitSig.S,
			cfg.ContractAddr(), // Operator is the FilecoinWarmStorageService contract
			rateAllowanceBig,
			lockupAllowanceBig,
			big.NewInt(int64(maxLockupPeriod)),
		)
		if err != nil {
			return fmt.Errorf("calling depositWithPermitAndApproveOperator: %w", err)
		}
	} else {
		// Call setOperatorApproval without deposit
		fmt.Println("Calling setOperatorApproval...")

		tx, err = paymentsContract.SetOperatorApproval(
			auth,
			cfg.TokenAddr(),
			cfg.ContractAddr(), // Operator is the FilecoinWarmStorageService contract
			true,               // Approve
			rateAllowanceBig,
			lockupAllowanceBig,
			big.NewInt(int64(maxLockupPeriod)),
		)
		if err != nil {
			return fmt.Errorf("calling setOperatorApproval: %w", err)
		}
	}

	receipt, err = contract.WaitForTransaction(ctx, client, tx.Hash())
	if err != nil {
		return fmt.Errorf("waiting for transaction: %w", err)
	}

	fmt.Println()
	fmt.Println("✓ Operator approval successful!")
	fmt.Printf("Transaction: %s\n", receipt.TxHash.Hex())
	fmt.Printf("Block: %d\n", receipt.BlockNumber.Uint64())
	fmt.Printf("Gas used: %d\n", receipt.GasUsed)

	if includeDeposit {
		fmt.Printf("\nDeposited %s tokens to your account in the Payments contract\n", amountBig.String())
	}

	fmt.Printf("\nFilecoinWarmStorageService contract (%s) is now approved as an operator\n", cfg.ContractAddress)
	fmt.Printf("  Rate allowance: %s\n", rateAllowanceBig.String())
	fmt.Printf("  Lockup allowance: %s\n", lockupAllowanceBig.String())
	fmt.Printf("  Max lockup period: %d seconds\n", maxLockupPeriod)

	return nil
}
