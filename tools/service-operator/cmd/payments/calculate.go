package payments

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/storacha/piri/tools/service-operator/internal/payments"
)

var (
	calcSize                string
	calcLockupDays          int
	calcMaxLockupPeriodDays int
	calcOutputFormat        string
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
  service-operator payments calculate --size 1TiB

  # Calculate for 2.5 TiB with 30 day lockup
  service-operator payments calculate --size 2.5TiB --lockup-days 30

  # Calculate for 500 GiB with custom max lockup period
  service-operator payments calculate --size 500GiB --lockup-days 15 --max-lockup-period-days 60

  # Output as shell-friendly format for scripting
  service-operator payments calculate --size 1TiB --format shell`,
	RunE: runCalculate,
}

func init() {
	calculateCmd.Flags().StringVar(&calcSize, "size", "", "Dataset size (e.g., 1TiB, 500GiB, 2.5TiB) (required)")
	calculateCmd.Flags().IntVar(&calcLockupDays, "lockup-days", payments.DefaultLockupDays, "Lockup period in days")
	calculateCmd.Flags().IntVar(&calcMaxLockupPeriodDays, "max-lockup-period-days", 30, "Maximum lockup period in days")
	calculateCmd.Flags().StringVar(&calcOutputFormat, "format", "human", "Output format: human, shell, or flags")

	cobra.MarkFlagRequired(calculateCmd.Flags(), "size")
}

func runCalculate(cobraCmd *cobra.Command, args []string) error {
	// Parse size
	sizeInBytes, err := payments.ParseSize(calcSize)
	if err != nil {
		return err
	}

	// Calculate allowances
	calc, err := payments.CalculateAllowances(sizeInBytes, calcLockupDays, calcMaxLockupPeriodDays)
	if err != nil {
		return fmt.Errorf("calculating allowances: %w", err)
	}

	// Output based on format
	switch calcOutputFormat {
	case "human":
		printHumanFormat(calc)
	case "shell":
		printShellFormat(calc)
	case "flags":
		printFlagsFormat(calc)
	default:
		return fmt.Errorf("unknown format: %s (supported: human, shell, flags)", calcOutputFormat)
	}

	return nil
}

func printHumanFormat(calc *payments.AllowanceCalculation) {
	fmt.Println("Operator Approval Allowance Calculation")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("Input Parameters:")
	fmt.Printf("  Dataset size:           %s (%s bytes)\n", payments.FormatSize(calc.SizeInBytes), calc.SizeInBytes.String())
	fmt.Printf("  Lockup period:          %d days (%d epochs)\n", calc.LockupDays, calc.LockupPeriodEpochs)
	fmt.Printf("  Max lockup period:      %d days (%s epochs)\n", calc.MaxLockupPeriodDays, calc.MaxLockupPeriod.String())
	fmt.Printf("  Storage price:          $5.00 USD per TiB/month\n")
	fmt.Println()
	fmt.Println("Calculated Allowances:")
	fmt.Printf("  Rate allowance:         %s base units/epoch (%s per epoch)\n",
		calc.RateAllowance.String(),
		payments.FormatTokenAmount(calc.RateAllowance, payments.TokenDecimals))
	fmt.Printf("  Lockup allowance:       %s base units (%s for %d days)\n",
		calc.LockupAllowance.String(),
		payments.FormatTokenAmount(calc.LockupAllowance, payments.TokenDecimals),
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