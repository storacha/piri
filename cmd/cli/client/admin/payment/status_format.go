package payment

import (
	"fmt"
	"math/big"
	"strings"
)

// formatTokenAmount formats a token amount (in wei, 18 decimals) to a human-readable string
// Shows dollar amounts for USDFC token
func formatTokenAmount(wei string) string {
	if wei == "" {
		return "$0.00"
	}
	weiInt, ok := new(big.Int).SetString(wei, 10)
	if !ok || weiInt.Sign() == 0 {
		return "$0.00"
	}

	// Convert to float with 18 decimals
	weiF := new(big.Float).SetInt(weiInt)
	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	result := new(big.Float).Quo(weiF, divisor)
	f, _ := result.Float64()

	if f >= 1000000 {
		return fmt.Sprintf("$%.2fM", f/1000000)
	} else if f >= 1000 {
		return fmt.Sprintf("$%.2fK", f/1000)
	} else if f >= 1 {
		return fmt.Sprintf("$%.2f", f)
	} else if f >= 0.01 {
		return fmt.Sprintf("$%.4f", f)
	}
	return fmt.Sprintf("$%.6f", f)
}

// formatTokenCompact formats a token amount compactly for table columns
func formatTokenCompact(wei string) string {
	if wei == "" {
		return "$0"
	}
	weiInt, ok := new(big.Int).SetString(wei, 10)
	if !ok || weiInt.Sign() == 0 {
		return "$0"
	}

	// Convert to float with 18 decimals
	weiF := new(big.Float).SetInt(weiInt)
	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	result := new(big.Float).Quo(weiF, divisor)
	f, _ := result.Float64()

	if f >= 1000 {
		return fmt.Sprintf("$%.1f", f)
	} else if f >= 1 {
		return fmt.Sprintf("$%.2f", f)
	}
	return fmt.Sprintf("$%.4f", f)
}

// formatEpoch formats an epoch number with thousands separators
func formatEpoch(epoch string) string {
	if epoch == "" {
		return "-"
	}
	epochInt, ok := new(big.Int).SetString(epoch, 10)
	if !ok {
		return epoch
	}
	return formatBigIntWithCommas(epochInt)
}

// formatBigIntWithCommas formats a big.Int with thousands separators
func formatBigIntWithCommas(n *big.Int) string {
	if n == nil {
		return "0"
	}
	s := n.String()
	if len(s) <= 3 {
		return s
	}

	var result strings.Builder
	start := len(s) % 3
	if start == 0 {
		start = 3
	}
	result.WriteString(s[:start])
	for i := start; i < len(s); i += 3 {
		result.WriteString(",")
		result.WriteString(s[i : i+3])
	}
	return result.String()
}

// formatRate formats a payment rate per epoch to a readable string
func formatRate(rateWei string) string {
	if rateWei == "" {
		return "$0/ep"
	}
	rateInt, ok := new(big.Int).SetString(rateWei, 10)
	if !ok || rateInt.Sign() == 0 {
		return "$0/ep"
	}

	// Convert to float with 18 decimals
	rateF := new(big.Float).SetInt(rateInt)
	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	result := new(big.Float).Quo(rateF, divisor)
	f, _ := result.Float64()

	if f >= 1 {
		return fmt.Sprintf("$%.2f/ep", f)
	} else if f >= 0.001 {
		return fmt.Sprintf("$%.4f/ep", f)
	}
	return fmt.Sprintf("$%.6f/ep", f)
}
