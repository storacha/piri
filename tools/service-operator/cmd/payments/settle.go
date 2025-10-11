package payments

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"

	"github.com/storacha/piri/tools/service-operator/eip712"
	"github.com/storacha/piri/tools/service-operator/internal/contract"
	"github.com/storacha/piri/tools/service-operator/internal/payments"
)

var (
	settleRailID     string
	settleUntilEpoch string
	settleAll        bool
)

var settleCmd = &cobra.Command{
	Use:   "settle",
	Short: "Settle payment rails to transfer locked funds",
	Long: `Settle payment rails to transfer locked funds from payer to payee.

Settlement triggers the validator (FilecoinWarmStorageService) to check which epochs have valid
PDP proofs and only pays for proven epochs. This moves funds from the payer's locked balance
to the payee's available balance in the Payments contract.

NETWORK_FEE: Settlement requires sending 0.0013 FIL as a network fee.

Examples:
  # Settle a specific rail to current epoch
  service-operator payments settle --rail-id 1

  # Settle a specific rail up to a specific epoch
  service-operator payments settle --rail-id 1 --until-epoch 1000000

  # Settle all rails for this service provider
  service-operator payments settle --all`,
	RunE: runSettle,
}

func init() {
	settleCmd.Flags().StringVar(&settleRailID, "rail-id", "", "Rail ID to settle")
	settleCmd.Flags().StringVar(&settleUntilEpoch, "until-epoch", "", "Settle up to this epoch (defaults to current block number)")
	settleCmd.Flags().BoolVar(&settleAll, "all", false, "Settle all rails for this service provider")
}

func runSettle(cobraCmd *cobra.Command, args []string) error {
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

	// Validate flags
	if !settleAll && settleRailID == "" {
		return fmt.Errorf("either --rail-id or --all must be specified")
	}
	if settleAll && settleRailID != "" {
		return fmt.Errorf("cannot specify both --rail-id and --all")
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

	serviceProviderAddr := crypto.PubkeyToAddress(privateKey.PublicKey)

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

	// Determine which rails to settle
	var railIDs []*big.Int
	if settleAll {
		// Query all rails for this service provider as payee
		rails, err := contract.QueryRailsForPayee(ctx, cfg.RPCUrl, cfg.PaymentsAddr(), serviceProviderAddr, cfg.TokenAddr())
		if err != nil {
			return fmt.Errorf("querying rails for payee: %w", err)
		}

		if len(rails) == 0 {
			fmt.Println("No payment rails found for this service provider.")
			return nil
		}

		// Filter only active (non-terminated) rails
		for _, rail := range rails {
			if !rail.IsTerminated {
				railIDs = append(railIDs, rail.RailId)
			}
		}

		if len(railIDs) == 0 {
			fmt.Println("No active payment rails found for this service provider.")
			return nil
		}

		fmt.Printf("Found %d active payment rail(s) to settle\n\n", len(railIDs))
	} else {
		// Parse single rail ID
		railID := new(big.Int)
		if _, ok := railID.SetString(settleRailID, 10); !ok {
			return fmt.Errorf("invalid rail ID: %s", settleRailID)
		}
		railIDs = []*big.Int{railID}
	}

	// Determine until epoch
	var untilEpoch *big.Int
	if settleUntilEpoch != "" {
		untilEpoch = new(big.Int)
		if _, ok := untilEpoch.SetString(settleUntilEpoch, 10); !ok {
			return fmt.Errorf("invalid until epoch: %s", settleUntilEpoch)
		}
	} else {
		// Default to current block number
		blockNumber, err := client.BlockNumber(ctx)
		if err != nil {
			return fmt.Errorf("getting current block number: %w", err)
		}
		untilEpoch = new(big.Int).SetUint64(blockNumber)
		fmt.Printf("Using current block number as until epoch: %s\n\n", untilEpoch.String())
	}

	// Settle each rail
	successCount := 0
	for _, railID := range railIDs {
		fmt.Printf("Settling rail %s...\n", railID.String())

		// Query rail info first
		railInfo, err := contract.QueryRailInfo(ctx, cfg.RPCUrl, cfg.PaymentsAddr(), railID)
		if err != nil {
			fmt.Printf("  ❌ Error querying rail info: %v\n\n", err)
			continue
		}

		fmt.Printf("  Payer:        %s\n", railInfo.From.Hex())
		fmt.Printf("  Payee:        %s\n", railInfo.To.Hex())
		fmt.Printf("  Operator:     %s\n", railInfo.Operator.Hex())
		fmt.Printf("  Settled up to: epoch %s\n", railInfo.SettledUpTo.String())
		fmt.Printf("  Payment rate: %s/epoch (%s/epoch)\n",
			railInfo.PaymentRate.String(),
			payments.FormatTokenAmount(railInfo.PaymentRate, decimals))

		// Create transaction auth
		auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
		if err != nil {
			fmt.Printf("  ❌ Error creating transaction auth: %v\n\n", err)
			continue
		}

		// Settle the rail
		result, err := contract.SettleRail(ctx, cfg.RPCUrl, cfg.PaymentsAddr(), auth, railID, untilEpoch)
		if err != nil {
			fmt.Printf("  ❌ Settlement failed: %v\n\n", err)
			continue
		}

		fmt.Printf("  ✓ Settlement successful!\n")
		fmt.Printf("  Transaction:  %s\n", result.TransactionHash.Hex())
		fmt.Printf("  Settled up to: epoch %s\n", result.FinalSettledEpoch.String())
		fmt.Printf("  Amount paid:  %s (%s)\n",
			result.TotalSettledAmount.String(),
			payments.FormatTokenAmount(result.TotalSettledAmount, decimals))
		fmt.Printf("  Payee received: %s (%s)\n",
			result.TotalNetPayeeAmount.String(),
			payments.FormatTokenAmount(result.TotalNetPayeeAmount, decimals))
		fmt.Printf("  Commission:   %s (%s)\n",
			result.TotalOperatorCommission.String(),
			payments.FormatTokenAmount(result.TotalOperatorCommission, decimals))
		if result.Note != "" {
			fmt.Printf("  Note:         %s\n", result.Note)
		}
		fmt.Println()

		successCount++
	}

	fmt.Printf("Settlement complete: %d of %d rail(s) settled successfully\n", successCount, len(railIDs))

	return nil
}
