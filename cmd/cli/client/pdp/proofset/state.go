package proofset

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/pdp/httpapi/client"
)

var (
	StateCmd = &cobra.Command{
		Use:   "state",
		Short: "Get state of a proof set",
		Args:  cobra.NoArgs,
		RunE:  doState,
	}
)

func init() {
	// TODO we can make this an arg instead
	StateCmd.Flags().Uint64(
		"proofset-id",
		0,
		"The proofset ID",
	)
	StateCmd.Flags().Bool(
		"json",
		false,
		"Output in JSON format",
	)
	cobra.CheckErr(StateCmd.MarkFlagRequired("proofset-id"))
}

func formatEpochTime(currentEpoch, targetEpoch int64) string {
	// Calculate the time difference in seconds (can be negative for past epochs)
	epochDiff := targetEpoch - currentEpoch
	seconds := epochDiff * 30

	// Calculate the estimated time
	estimatedTime := time.Now().Add(time.Duration(seconds) * time.Second)
	return fmt.Sprintf("(est. %s)", estimatedTime.Format("2006-01-02 15:04:05"))
}

func formatEpochTimeWithRelative(currentEpoch, targetEpoch int64) string {
	// Calculate the time difference in seconds (can be negative for past epochs)
	epochDiff := targetEpoch - currentEpoch
	seconds := epochDiff * 30

	// Calculate the estimated time
	estimatedTime := time.Now().Add(time.Duration(seconds) * time.Second)
	timeStr := estimatedTime.Format("2006-01-02 15:04:05")

	// Add relative time description
	if epochDiff == 0 {
		return fmt.Sprintf("(est. %s, now)", timeStr)
	} else if epochDiff < 0 {
		// Past epoch
		return fmt.Sprintf("(est. %s, %s ago)", timeStr, formatDuration(-seconds))
	} else {
		// Future epoch
		return fmt.Sprintf("(est. %s, in %s)", timeStr, formatDuration(seconds))
	}
}

func formatDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%d seconds", seconds)
	}

	minutes := seconds / 60
	if minutes < 60 {
		if seconds%60 == 0 {
			return fmt.Sprintf("%d minutes", minutes)
		}
		return fmt.Sprintf("%d min %d sec", minutes, seconds%60)
	}

	hours := minutes / 60
	remainingMinutes := minutes % 60
	if hours < 24 {
		if remainingMinutes == 0 {
			return fmt.Sprintf("%d hours", hours)
		}
		return fmt.Sprintf("%d hr %d min", hours, remainingMinutes)
	}

	days := hours / 24
	remainingHours := hours % 24
	if remainingHours == 0 {
		return fmt.Sprintf("%d days", days)
	}
	return fmt.Sprintf("%d days %d hr", days, remainingHours)
}

func formatEpochDuration(epochs int64) string {
	if epochs == 0 {
		return "(0 seconds)"
	}

	seconds := epochs * 30

	if seconds < 60 {
		return fmt.Sprintf("(%d seconds)", seconds)
	}

	minutes := seconds / 60
	if minutes < 60 {
		if seconds%60 == 0 {
			return fmt.Sprintf("(%d minutes)", minutes)
		}
		return fmt.Sprintf("(%d min %d sec)", minutes, seconds%60)
	}

	hours := minutes / 60
	remainingMinutes := minutes % 60
	if hours < 24 {
		if remainingMinutes == 0 {
			return fmt.Sprintf("(%d hours)", hours)
		}
		return fmt.Sprintf("(%d hr %d min)", hours, remainingMinutes)
	}

	days := hours / 24
	remainingHours := hours % 24
	if remainingHours == 0 {
		return fmt.Sprintf("(%d days)", days)
	}
	return fmt.Sprintf("(%d days %d hr)", days, remainingHours)
}

func formatRelativeTime(currentEpoch, targetEpoch int64) string {
	if targetEpoch <= currentEpoch {
		return "(past)"
	}

	epochDiff := targetEpoch - currentEpoch
	seconds := epochDiff * 30

	if seconds < 60 {
		return fmt.Sprintf("(~%d seconds)", seconds)
	}

	minutes := seconds / 60
	if minutes < 60 {
		if seconds%60 == 0 {
			return fmt.Sprintf("(~%d minutes)", minutes)
		}
		return fmt.Sprintf("(~%d min %d sec)", minutes, seconds%60)
	}

	hours := minutes / 60
	remainingMinutes := minutes % 60
	if hours < 24 {
		if remainingMinutes == 0 {
			return fmt.Sprintf("(~%d hours)", hours)
		}
		return fmt.Sprintf("(~%d hr %d min)", hours, remainingMinutes)
	}

	days := hours / 24
	remainingHours := hours % 24
	if remainingHours == 0 {
		return fmt.Sprintf("(~%d days)", days)
	}
	return fmt.Sprintf("(~%d days %d hr)", days, remainingHours)
}

func formatTokenAmount(attoFil *big.Int) string {
	if attoFil == nil || attoFil.Cmp(big.NewInt(0)) == 0 {
		return "0 FIL"
	}

	type unit struct {
		name     string
		decimals int
	}

	units := []unit{
		{"FIL", 18},
		{"milliFIL", 15},
		{"microFIL", 12},
		{"nanoFIL", 9},
		{"picoFIL", 6},
		{"femtoFIL", 3},
		{"attoFIL", 0},
	}

	for _, u := range units {
		divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(u.decimals)), nil)
		quotient := new(big.Float).SetInt(attoFil)
		divisorFloat := new(big.Float).SetInt(divisor)
		result := new(big.Float).Quo(quotient, divisorFloat)

		unitValue, _ := result.Float64()
		if unitValue >= 1 {
			decimals := 2
			if u.name == "FIL" {
				decimals = 4
			}
			format := fmt.Sprintf("%%.%df %%s", decimals)
			return fmt.Sprintf(format, unitValue, u.name)
		}
	}

	return "0 FIL"
}

func doState(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load[config.Client]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	api, err := client.NewFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	proofSetID, err := cmd.Flags().GetUint64("proofset-id")
	if err != nil {
		return fmt.Errorf("parsing proofset ID: %w", err)
	}

	proofSet, err := api.GetProofSetState(ctx, proofSetID)
	if err != nil {
		return fmt.Errorf("getting proof set status: %w", err)
	}

	jsonOutput, err := cmd.Flags().GetBool("json")
	if err != nil {
		return fmt.Errorf("parsing json flag: %w", err)
	}

	if jsonOutput {
		jsonProofSet, err := json.MarshalIndent(proofSet, "", "  ")
		if err != nil {
			return fmt.Errorf("rendering json: %w", err)
		}
		fmt.Print(string(jsonProofSet))
		return nil
	}

	// Table formatted output using cmd for printing
	cmd.Println("╔═══════════════════════════════════════════════════════════════╗")
	cmd.Println("║                        PROOF SET STATE                        ║")
	cmd.Println("╚═══════════════════════════════════════════════════════════════╝")
	cmd.Println()
	cmd.Println("Note: Timestamps are estimated based on current epoch alignment with system time (30-second epochs).")
	cmd.Println()

	// Immutable configuration (these don't change after creation)
	cmd.Println("CONFIGURATION")
	cmd.Println("─────────────────────────")
	cmd.Printf("  Proof Set ID:            %d\n", proofSet.ID)
	cmd.Printf("  Proving Period:          %d epochs %s\n", proofSet.ProvingPeriod, formatEpochDuration(proofSet.ProvingPeriod))
	cmd.Printf("  Challenge Window:        %d epochs %s\n", proofSet.ChallengeWindow, formatEpochDuration(proofSet.ChallengeWindow))

	if len(proofSet.ContractState.Owners) > 0 {
		cmd.Print("  Owners:                  ")
		for i, owner := range proofSet.ContractState.Owners {
			if i == 0 {
				cmd.Printf("%s\n", owner.Hex())
			} else {
				cmd.Printf("                           %s\n", owner.Hex())
			}
		}
	} else {
		cmd.Println("  Owners:                  (none)")
	}
	cmd.Printf("  Initialized:             %v\n", proofSet.Initialized)
	cmd.Println()

	// System's view of the chain state (local node's perspective)
	cmd.Println("SYSTEM VIEW (Local Node)")
	cmd.Println("────────────────────────")
	cmd.Printf("  Current Epoch:           %d %s\n", proofSet.CurrentEpoch, formatEpochTime(proofSet.CurrentEpoch, proofSet.CurrentEpoch))
	cmd.Printf("  Next Challenge Epoch:    %d %s\n", proofSet.NextChallengeEpoch, formatEpochTimeWithRelative(proofSet.CurrentEpoch, proofSet.NextChallengeEpoch))
	cmd.Printf("  Previous Challenge:      %d %s\n", proofSet.PreviousChallengeEpoch, formatEpochTimeWithRelative(proofSet.CurrentEpoch, proofSet.PreviousChallengeEpoch))
	cmd.Println()
	cmd.Println("  Status:")
	cmd.Printf("    • Challenge Issued:    %v\n", proofSet.ChallengedIssued)
	cmd.Printf("    • In Challenge Window: %v", proofSet.InChallengeWindow)
	if proofSet.InChallengeWindow {
		// Calculate when challenge window ends
		challengeWindowEnd := proofSet.NextChallengeEpoch + proofSet.ChallengeWindow
		cmd.Printf(" (ends epoch %d %s)", challengeWindowEnd, formatEpochTimeWithRelative(proofSet.CurrentEpoch, challengeWindowEnd))
	}
	cmd.Println()
	cmd.Printf("    • In Fault State:      %v\n", proofSet.IsInFaultState)
	cmd.Printf("    • Has Proven:          %v\n", proofSet.HasProven)
	cmd.Printf("    • Is Proving:          %v\n", proofSet.IsProving)
	cmd.Println()

	// Contract's on-chain state (may differ from system view when out of sync)
	cmd.Println("CONTRACT STATE (On-Chain)")
	cmd.Println("─────────────────────────")
	cmd.Printf("  Next Challenge Window:   %d %s\n", proofSet.ContractState.NextChallengeWindowStart, formatEpochTimeWithRelative(proofSet.CurrentEpoch, int64(proofSet.ContractState.NextChallengeWindowStart)))
	cmd.Printf("  Next Challenge Epoch:    %d %s\n", proofSet.ContractState.NextChallengeEpoch, formatEpochTimeWithRelative(proofSet.CurrentEpoch, int64(proofSet.ContractState.NextChallengeEpoch)))
	cmd.Printf("  Max Proving Period:      %d epochs %s\n", proofSet.ContractState.MaxProvingPeriod, formatEpochDuration(int64(proofSet.ContractState.MaxProvingPeriod)))
	cmd.Printf("  Challenge Window:        %d epochs %s\n", proofSet.ContractState.ChallengeWindow, formatEpochDuration(int64(proofSet.ContractState.ChallengeWindow)))
	cmd.Printf("  Challenge Range:         %d\n", proofSet.ContractState.ChallengeRange)
	cmd.Println()
	cmd.Println("  Fees:")
	cmd.Printf("    • Proof Fee:           %s\n", formatTokenAmount(new(big.Int).SetUint64(proofSet.ContractState.ProofFee)))
	cmd.Printf("    • Buffered Fee:        %s\n", formatTokenAmount(new(big.Int).SetUint64(proofSet.ContractState.ProofFeeBuffered)))

	if len(proofSet.ContractState.ScheduledRemovals) > 0 {
		cmd.Println()
		var removals []string
		for _, removal := range proofSet.ContractState.ScheduledRemovals {
			removals = append(removals, fmt.Sprintf("%d", removal))
		}
		cmd.Printf("  Scheduled Removals:      [%s]\n", strings.Join(removals, ", "))
	}

	// Add a note about potential sync issues
	if proofSet.IsInFaultState {
		cmd.Println()
		cmd.Println("⚠️  WARNING: Node is in fault state. System view may be out of sync with contract.")
	}

	// Show when next proving opportunity
	if !proofSet.IsInFaultState && proofSet.NextChallengeEpoch > proofSet.CurrentEpoch {
		cmd.Println()
		epochsUntilChallenge := proofSet.NextChallengeEpoch - proofSet.CurrentEpoch
		cmd.Printf("🕑 Next proving opportunity in %d epochs %s %s\n", epochsUntilChallenge, formatRelativeTime(proofSet.CurrentEpoch, proofSet.NextChallengeEpoch), formatEpochTime(proofSet.CurrentEpoch, proofSet.NextChallengeEpoch))
	}

	return nil

}
