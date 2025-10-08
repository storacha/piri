package payments

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

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
	depositAmountFlag   string
	depositToFlag       string
	depositDeadlineFlag uint64
)

var depositCmd = &cobra.Command{
	Use:   "deposit",
	Short: "Deposit tokens into the Payments contract",
	Long: `Deposit ERC20 tokens into your account in the Payments contract using EIP-2612 permit.

The EIP-2612 permit signature is generated internally using your private key, allowing
gasless token approval. The deposit is credited to your address (or a specified recipient).

After depositing, you can approve operators to manage payment rails on your behalf.

Examples:
  # Deposit 10 USDFC to your own account
  service-operator payments deposit --amount 10000000

  # Deposit to a specific address
  service-operator payments deposit \
    --amount 10000000 \
    --to 0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb

  # Deposit with custom permit deadline
  service-operator payments deposit \
    --amount 10000000 \
    --permit-deadline 1704067200`,
	RunE: runDeposit,
}

func init() {
	depositCmd.Flags().StringVar(&depositAmountFlag, "amount", "", "Amount to deposit in base token units (required)")
	depositCmd.Flags().StringVar(&depositToFlag, "to", "", "Address to credit the deposit (default: your address)")
	depositCmd.Flags().Uint64Var(&depositDeadlineFlag, "permit-deadline", 0, "Permit deadline as unix timestamp (default: 1 hour from now)")

	cobra.MarkFlagRequired(depositCmd.Flags(), "amount")
}

func runDeposit(cobraCmd *cobra.Command, args []string) error {
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

	// Parse deposit amount
	amountBig := new(big.Int)
	if _, ok := amountBig.SetString(depositAmountFlag, 10); !ok {
		return fmt.Errorf("invalid amount: %s (must be a valid number)", depositAmountFlag)
	}

	if amountBig.Sign() <= 0 {
		return fmt.Errorf("amount must be greater than 0")
	}

	// Set default permit deadline if not provided
	if depositDeadlineFlag == 0 {
		depositDeadlineFlag = uint64(time.Now().Add(1 * time.Hour).Unix())
	}

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

	// Derive owner address from private key
	ownerAddr := crypto.PubkeyToAddress(privateKey.PublicKey)

	// Determine recipient address
	var toAddr common.Address
	if depositToFlag != "" {
		if !common.IsHexAddress(depositToFlag) {
			return fmt.Errorf("invalid --to address: %s", depositToFlag)
		}
		toAddr = common.HexToAddress(depositToFlag)
	} else {
		toAddr = ownerAddr // Default to depositing to self
	}

	fmt.Println("Depositing tokens into Payments contract")
	fmt.Printf("Payments contract:  %s\n", cfg.PaymentsContractAddress)
	fmt.Printf("Token contract:     %s\n", cfg.TokenContractAddress)
	fmt.Printf("From (owner):       %s\n", ownerAddr.Hex())
	fmt.Printf("To (recipient):     %s\n", toAddr.Hex())
	fmt.Printf("Amount:             %s base units\n", amountBig.String())
	fmt.Printf("RPC URL:            %s\n", cfg.RPCUrl)
	fmt.Println()

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

	auth, err := contract.CreateTransactor(ctx, client, privateKey)
	if err != nil {
		return fmt.Errorf("creating transactor: %w", err)
	}

	// Get chain ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("getting chain ID: %w", err)
	}

	// Generate EIP-2612 permit signature
	fmt.Println("Generating EIP-2612 permit signature...")
	permitSig, err := eip712.GeneratePermitSignature(
		ctx,
		client,
		cfg.TokenAddr(),
		cfg.PaymentsAddr(), // Spender is the Payments contract
		amountBig,
		big.NewInt(int64(depositDeadlineFlag)),
		privateKey,
		chainID,
	)
	if err != nil {
		return fmt.Errorf("generating permit signature: %w", err)
	}

	paymentsContract, err := bindings.NewPayments(cfg.PaymentsAddr(), client)
	if err != nil {
		return fmt.Errorf("creating payments contract binding: %w", err)
	}

	fmt.Println("Calling depositWithPermit...")

	// Call depositWithPermit
	tx, err := paymentsContract.DepositWithPermit(
		auth,
		cfg.TokenAddr(),
		toAddr,
		amountBig,
		big.NewInt(int64(depositDeadlineFlag)),
		permitSig.V,
		permitSig.R,
		permitSig.S,
	)
	if err != nil {
		return fmt.Errorf("calling depositWithPermit: %w", err)
	}

	receipt, err := contract.WaitForTransaction(ctx, client, tx.Hash())
	if err != nil {
		return fmt.Errorf("waiting for transaction: %w", err)
	}

	fmt.Println()
	fmt.Println("✓ Deposit successful!")
	fmt.Printf("Transaction:        %s\n", receipt.TxHash.Hex())
	fmt.Printf("Block:              %d\n", receipt.BlockNumber.Uint64())
	fmt.Printf("Gas used:           %d\n", receipt.GasUsed)
	fmt.Println()
	fmt.Printf("Deposited %s tokens to %s\n", amountBig.String(), toAddr.Hex())
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Approve the FilecoinWarmStorageService contract as an operator:")
	fmt.Println("     service-operator payments approve-service \\")
	fmt.Println("       --rate-allowance <value> \\")
	fmt.Println("       --lockup-allowance <value> \\")
	fmt.Println("       --max-lockup-period <value>")
	fmt.Println()
	fmt.Println("  2. Use 'payments calculate' to determine allowance values:")
	fmt.Println("     service-operator payments calculate --size <dataset-size>")

	return nil
}
