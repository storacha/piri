package payments

import (
	"fmt"
	"math/big"

	"github.com/dustin/go-humanize"
)

// Constants from the payment rate specification
const (
	// Size constants
	TiBInBytes = 1_099_511_627_776 // 1024^4
	GiBInBytes = 1_073_741_824     // 1024^3
	MiBInBytes = 1_048_576         // 1024^2

	// Epoch constants
	EpochsPerDay = 2_880

	// Default values
	DefaultLockupDays = 10
	// DefaultMaxLockupPeriodEpochs is 30 days (will be calculated as 30 * EpochsPerDay)
	DefaultMaxLockupPeriodDays = 30
)

// AllowanceCalculation holds the calculated allowance values
type AllowanceCalculation struct {
	// Input parameters
	SizeInBytes         *big.Int
	LockupDays          int
	MaxLockupPeriodDays int

	// Calculated values
	RateAllowance   *big.Int // tokens per epoch
	LockupAllowance *big.Int // total tokens
	MaxLockupPeriod *big.Int // in epochs

	// Intermediate values for display
	LockupPeriodEpochs int64
	RatePerEpoch       *big.Int
}

// ParseSize parses human-readable size strings like "1TiB", "500GiB", "1.5TiB" to bytes
// Uses go-humanize to parse sizes (supports TB, GB, MB, KB or TiB, GiB, MiB, KiB)
func ParseSize(sizeStr string) (*big.Int, error) {
	bytes, err := humanize.ParseBytes(sizeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid size format: %s (expected format: 1TiB, 500GiB, 1.5TiB): %w", sizeStr, err)
	}

	result := new(big.Int)
	result.SetUint64(bytes)

	return result, nil
}

// CalculateAllowances calculates the rate allowance, lockup allowance, and max lockup period
// based on the dataset size and lockup parameters.
//
// Formula from https://filecoinproject.slack.com/archives/C07CGTXHHT4/p1759276539956319
//   rateAllowance = (sizeInBytes × pricePerTiBPerMonth) / (TiB_IN_BYTES × epochsPerMonth)
//   lockupAllowance = ratePerEpoch × lockupPeriodInEpochs
//   maxLockupPeriod = maxLockupPeriodDays × EpochsPerDay
//
// Parameters:
//   - sizeInBytes: The dataset size in bytes
//   - lockupDays: The lockup period in days
//   - maxLockupPeriodDays: The maximum lockup period in days
//   - pricePerTiBPerMonth: The price per TiB per month in base token units (queried from contract)
//   - epochsPerMonth: The number of epochs per month (queried from contract)
func CalculateAllowances(sizeInBytes *big.Int, lockupDays int, maxLockupPeriodDays int, pricePerTiBPerMonth *big.Int, epochsPerMonth uint64) (*AllowanceCalculation, error) {
	if sizeInBytes == nil || sizeInBytes.Sign() <= 0 {
		return nil, fmt.Errorf("size must be greater than 0")
	}
	if lockupDays <= 0 {
		return nil, fmt.Errorf("lockup days must be greater than 0")
	}
	if maxLockupPeriodDays <= 0 {
		return nil, fmt.Errorf("max lockup period days must be greater than 0")
	}
	if pricePerTiBPerMonth == nil || pricePerTiBPerMonth.Sign() <= 0 {
		return nil, fmt.Errorf("price per TiB per month must be greater than 0")
	}
	if epochsPerMonth == 0 {
		return nil, fmt.Errorf("epochs per month must be greater than 0")
	}

	// Calculate rate per epoch
	// rateAllowance = (sizeInBytes × pricePerTiBPerMonth) / (TiB_IN_BYTES × epochsPerMonth)
	// Use ceiling division to ensure small datasets get at least 1 base unit per epoch

	numerator := new(big.Int).Mul(sizeInBytes, pricePerTiBPerMonth)
	denominator := new(big.Int).Mul(big.NewInt(TiBInBytes), big.NewInt(int64(epochsPerMonth)))

	ratePerEpoch := new(big.Int)
	remainder := new(big.Int)
	ratePerEpoch.DivMod(numerator, denominator, remainder)

	// Round up if there's a remainder (ceiling division)
	if remainder.Sign() > 0 {
		ratePerEpoch.Add(ratePerEpoch, big.NewInt(1))
	}

	// Calculate lockup period in epochs
	lockupPeriodEpochs := int64(lockupDays) * EpochsPerDay

	// Calculate lockup allowance
	// lockupAllowance = ratePerEpoch × lockupPeriodInEpochs
	lockupAllowance := new(big.Int).Mul(ratePerEpoch, big.NewInt(lockupPeriodEpochs))

	// Calculate max lockup period in epochs
	maxLockupPeriodEpochs := int64(maxLockupPeriodDays) * EpochsPerDay

	return &AllowanceCalculation{
		SizeInBytes:         new(big.Int).Set(sizeInBytes),
		LockupDays:          lockupDays,
		MaxLockupPeriodDays: maxLockupPeriodDays,
		RateAllowance:       ratePerEpoch,
		LockupAllowance:     lockupAllowance,
		MaxLockupPeriod:     big.NewInt(maxLockupPeriodEpochs),
		LockupPeriodEpochs:  lockupPeriodEpochs,
		RatePerEpoch:        new(big.Int).Set(ratePerEpoch),
	}, nil
}

// FormatSize formats bytes into human-readable size with appropriate unit
// Uses go-humanize to format in IEC format (KiB, MiB, GiB, TiB)
func FormatSize(bytes *big.Int) string {
	if bytes == nil || bytes.Sign() == 0 {
		return "0 B"
	}

	// Convert big.Int to uint64 for humanize
	// Note: This will overflow for very large values, but that's acceptable
	// for display purposes in this context
	if !bytes.IsUint64() {
		return fmt.Sprintf("%s bytes (too large to format)", bytes.String())
	}

	return humanize.IBytes(bytes.Uint64())
}

// FormatTokenAmount formats token base units as human-readable USD amount
// decimals: the number of decimals the token uses (e.g., 6 for USDC, 18 for standard ERC20)
// For USDFC with 6 decimals: 1,000,000 base units = $1.00 USD
// For standard ERC20 with 18 decimals: 1,000,000,000,000,000,000 base units = $1.00 USD
func FormatTokenAmount(baseUnits *big.Int, decimals uint8) string {
	if baseUnits == nil || baseUnits.Sign() == 0 {
		return "$0.00"
	}

	// Convert to float for display
	// divisor = 10^decimals
	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))
	amount := new(big.Float).SetInt(baseUnits)
	usd := new(big.Float).Quo(amount, divisor)

	// Format with appropriate precision
	val, _ := usd.Float64()
	if val < 0.01 {
		// For very small amounts, show more precision
		return fmt.Sprintf("$%.18f", val)
	}
	return fmt.Sprintf("$%.18f", val)
}
