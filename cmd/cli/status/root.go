package status

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/storacha/piri/pkg/client"
)

var Cmd = &cobra.Command{
	Use:   "status",
	Short: "Check node status and health",
	Long: `Check the status of a running piri node.

This command connects to the local piri service and reports on its current state,
including whether it's safe to perform operations like updates.`,
	RunE: runStatus,
}

var (
	jsonOutput bool
)

func init() {
	Cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	Cmd.SetOut(os.Stdout)
	Cmd.SetErr(os.Stderr)

	// Add subcommands
	Cmd.AddCommand(upgradeCheckCmd)
}

func runStatus(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	status, err := client.GetNodeStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to get node status: %w", err)
	}

	if jsonOutput {
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(status)
	}

	// Human-readable output
	cmd.Println("Node Status")
	cmd.Println("===========")
	cmd.Printf("Healthy:            %s\n", formatBool(status.Healthy))
	cmd.Printf("Currently Proving:  %s\n", formatBool(status.IsProving))
	cmd.Printf("In Challenge:       %s\n", formatBool(status.InChallengeWindow))
	cmd.Printf("Has Proven:         %s\n", formatBool(status.HasProven))
	cmd.Printf("In Fault State:     %s\n", formatBool(status.InFaultState))
	cmd.Printf("Safe to Update:     %s\n", formatBool(status.UpgradeSafe))

	if status.NextChallenge != nil {
		cmd.Printf("Next Challenge:     %s\n", status.NextChallenge.Format("2006-01-02 15:04:05 MST"))
	}

	// Add explanation if update is not safe
	if !status.UpgradeSafe {
		cmd.Println()
		if status.IsProving {
			cmd.Println("⚠️  Node is currently generating a proof. Updates should wait.")
		} else if status.InChallengeWindow && !status.HasProven {
			cmd.Println("⚠️  Node is in a challenge window but has not proven yet. Updates should wait.")
		}
	}

	return nil
}

func formatBool(b bool) string {
	if b {
		return "YES"
	}
	return "NO"
}