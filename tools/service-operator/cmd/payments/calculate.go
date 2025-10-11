package payments

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
	"github.com/storacha/piri/tools/service-operator/internal/config"
	"github.com/storacha/piri/tools/service-operator/internal/contract"
	"github.com/storacha/piri/tools/service-operator/internal/payments"
)

var (
	calcSize                string
	calcLockupDays          int
	calcMaxLockupPeriodDays int
	calcOutputFormat        string

	// Manual override flags (for testing/debugging)
	calcTokenDecimals       int
	calcPricePerTiBPerMonth string
	calcEpochsPerMonth      uint64
)

var calculateCmd = &cobra.Command{
	Use:   "calculate",
	Short: "Calculate operator approval allowances from dataset size",
	Long: `Calculate the rate allowance, lockup allowance, and max lockup period values
needed for the approve-service command based on your dataset size and lockup preferences.

The calculated values can be copied directly into the approve-service command flags.

Formula:
  rateAllowance = (sizeInBytes × 5) / (1TiB × 86,400 epochs)
  lockupAllowance = rateAllowance × (lockupDays × 2,880 epochs/day)
  maxLockupPeriod = maxLockupPeriodDays × 2,880 epochs/day

Examples:
  # Calculate for 1 TiB with default 10 day lockup
  service-operator payments calculate --size 1TiB --network calibration

  # Calculate for 2.5 TiB with 30 day lockup
  service-operator payments calculate --size 2.5TiB --lockup-days 30 \
    --rpc-url https://api.calibration.node.glif.io/rpc/v1 \
    --contract-address 0x60F412Fd67908a38A5E05C54905daB923413EEA6

  # Calculate for 500 GiB with custom max lockup period
  service-operator payments calculate --size 500GiB --lockup-days 15 --max-lockup-period-days 60

  # Output as shell-friendly format for scripting
  service-operator payments calculate --size 1TiB --format shell`,
	RunE: runCalculate,
}

func init() {
	calculateCmd.Flags().StringVar(&calcSize, "size", "", "Dataset size (e.g., 1TiB, 500GiB, 2.5TiB) (required)")
	calculateCmd.Flags().IntVar(&calcLockupDays, "lockup-days", payments.DefaultLockupDays, "Lockup period in days")
	calculateCmd.Flags().IntVar(&calcMaxLockupPeriodDays, "max-lockup-period-days", payments.DefaultMaxLockupPeriodDays, "Maximum lockup period in days")
	calculateCmd.Flags().StringVar(&calcOutputFormat, "format", "human", "Output format: human, shell, or flags")

	// Manual override flags (for testing/debugging)
	calculateCmd.Flags().IntVar(&calcTokenDecimals, "token-decimals", -1, "Token decimals (optional, overrides contract query)")
	calculateCmd.Flags().StringVar(&calcPricePerTiBPerMonth, "price-per-tib-per-month", "", "Price per TiB per month in base units (optional, overrides contract query)")
	calculateCmd.Flags().Uint64Var(&calcEpochsPerMonth, "epochs-per-month", 0, "Epochs per month (optional, overrides contract query)")

	cobra.MarkFlagRequired(calculateCmd.Flags(), "size")
}

func runCalculate(cobraCmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Parse size
	sizeInBytes, err := payments.ParseSize(calcSize)
	if err != nil {
		return err
	}

	// Determine parameters (query from contract or use manual overrides)
	pricePerTiBPerMonth, epochsPerMonth, tokenDecimals, parametersSource, err := determineParameters(ctx, cfg)
	if err != nil {
		return fmt.Errorf("determining parameters: %w", err)
	}

	// Calculate allowances
	calc, err := payments.CalculateAllowances(sizeInBytes, calcLockupDays, calcMaxLockupPeriodDays, pricePerTiBPerMonth, epochsPerMonth)
	if err != nil {
		return fmt.Errorf("calculating allowances: %w", err)
	}

	// Output based on format
	switch calcOutputFormat {
	case "human":
		printHumanFormat(calc, tokenDecimals, parametersSource)
	case "shell":
		printShellFormat(calc)
	case "flags":
		printFlagsFormat(calc)
	default:
		return fmt.Errorf("unknown format: %s (supported: human, shell, flags)", calcOutputFormat)
	}

	return nil
}

// determineParameters determines the pricing parameters either from contract queries or manual overrides
// Returns: pricePerTiBPerMonth, epochsPerMonth, tokenDecimals, source, error
func determineParameters(ctx context.Context, cfg config.Config) (*big.Int, uint64, uint8, string, error) {
	// Check if all manual overrides are provided
	if calcPricePerTiBPerMonth != "" && calcEpochsPerMonth != 0 && calcTokenDecimals >= 0 {
		price, ok := new(big.Int).SetString(calcPricePerTiBPerMonth, 10)
		if !ok {
			return nil, 0, 0, "", fmt.Errorf("invalid price format: %s", calcPricePerTiBPerMonth)
		}
		return price, calcEpochsPerMonth, uint8(calcTokenDecimals), "manual overrides", nil
	}

	// Query from contract
	// Require explicit RPC URL and service contract address
	if cfg.RPCUrl == "" {
		return nil, 0, 0, "", fmt.Errorf("--rpc-url is required (or provide manual overrides)")
	}

	if cfg.ContractAddress == "" {
		return nil, 0, 0, "", fmt.Errorf("--contract-address is required (or provide manual overrides)")
	}

	rpcURL := cfg.RPCUrl
	serviceContractAddr := cfg.ContractAddress

	fmt.Printf("Querying contract parameters...\n")
	fmt.Printf("  RPC URL: %s\n", rpcURL)
	fmt.Printf("  Service Contract: %s\n", serviceContractAddr)

	// Query service pricing
	pricing, err := contract.QueryServicePrice(ctx, rpcURL, common.HexToAddress(serviceContractAddr))
	if err != nil {
		return nil, 0, 0, "", fmt.Errorf("querying service price: %w", err)
	}

	fmt.Printf("  Token Address: %s\n", pricing.TokenAddress.Hex())

	// Use queried price or manual override
	pricePerTiBPerMonth := pricing.PricePerTiBPerMonthNoCDN
	if calcPricePerTiBPerMonth != "" {
		price, ok := new(big.Int).SetString(calcPricePerTiBPerMonth, 10)
		if !ok {
			return nil, 0, 0, "", fmt.Errorf("invalid price format: %s", calcPricePerTiBPerMonth)
		}
		pricePerTiBPerMonth = price
	}

	// Use queried epochs or manual override
	epochsPerMonth := pricing.EpochsPerMonth.Uint64()
	if calcEpochsPerMonth != 0 {
		epochsPerMonth = calcEpochsPerMonth
	}

	// Query token decimals or use manual override
	var tokenDecimals uint8
	if calcTokenDecimals >= 0 {
		tokenDecimals = uint8(calcTokenDecimals)
	} else {
		// Determine token address (queried or from config)
		tokenAddr := pricing.TokenAddress
		if cfg.TokenContractAddress != "" {
			tokenAddr = common.HexToAddress(cfg.TokenContractAddress)
		}

		decimals, err := contract.QueryTokenDecimals(ctx, rpcURL, tokenAddr)
		if err != nil {
			return nil, 0, 0, "", fmt.Errorf("querying token decimals: %w", err)
		}
		tokenDecimals = decimals
	}

	fmt.Printf("\n")

	return pricePerTiBPerMonth, epochsPerMonth, tokenDecimals, "queried from contract", nil
}

func printHumanFormat(calc *payments.AllowanceCalculation, tokenDecimals uint8, parametersSource string) {
	fmt.Println("Operator Approval Allowance Calculation")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Printf("Parameters Source: %s\n", parametersSource)
	fmt.Printf("Token Decimals: %d\n", tokenDecimals)
	fmt.Println()
	fmt.Println("Input Parameters:")
	fmt.Printf("  Dataset size:           %s (%s bytes)\n", payments.FormatSize(calc.SizeInBytes), calc.SizeInBytes.String())
	fmt.Printf("  Lockup period:          %d days (%d epochs)\n", calc.LockupDays, calc.LockupPeriodEpochs)
	fmt.Printf("  Max lockup period:      %d days (%s epochs)\n", calc.MaxLockupPeriodDays, calc.MaxLockupPeriod.String())
	fmt.Println()
	fmt.Println("Calculated Allowances:")
	fmt.Printf("  Rate allowance:         %s base units/epoch (%s per epoch)\n",
		calc.RateAllowance.String(),
		payments.FormatTokenAmount(calc.RateAllowance, tokenDecimals))
	fmt.Printf("  Lockup allowance:       %s base units (%s for %d days)\n",
		calc.LockupAllowance.String(),
		payments.FormatTokenAmount(calc.LockupAllowance, tokenDecimals),
		calc.LockupDays)
	fmt.Printf("  Max lockup period:      %s epochs (%d days)\n",
		calc.MaxLockupPeriod.String(),
		calc.MaxLockupPeriodDays)
	fmt.Println()
	fmt.Println("Usage with approve-service:")
	fmt.Println("  Copy these exact base unit values to the command:")
	fmt.Println()
	fmt.Printf("  service-operator payments approve-service \\\n")
	fmt.Printf("    --rate-allowance %s \\\n", calc.RateAllowance.String())
	fmt.Printf("    --lockup-allowance %s \\\n", calc.LockupAllowance.String())
	fmt.Printf("    --max-lockup-period %s\n", calc.MaxLockupPeriod.String())
	fmt.Println()
}

func printShellFormat(calc *payments.AllowanceCalculation) {
	fmt.Printf("RATE_ALLOWANCE=%s\n", calc.RateAllowance.String())
	fmt.Printf("LOCKUP_ALLOWANCE=%s\n", calc.LockupAllowance.String())
	fmt.Printf("MAX_LOCKUP_PERIOD=%s\n", calc.MaxLockupPeriod.String())
}

func printFlagsFormat(calc *payments.AllowanceCalculation) {
	fmt.Printf("--rate-allowance %s --lockup-allowance %s --max-lockup-period %s\n",
		calc.RateAllowance.String(),
		calc.LockupAllowance.String(),
		calc.MaxLockupPeriod.String())
}