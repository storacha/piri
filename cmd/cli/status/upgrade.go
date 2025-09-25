package status

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/storacha/piri/pkg/client"
)

var upgradeCheckCmd = &cobra.Command{
	Use:   "upgrade-check",
	Short: "Check if it's safe to upgrade",
	Long: `Check if the node is in a state where it's safe to perform an upgrade.

Exit codes:
  0 - Safe to upgrade
  1 - Not safe to upgrade
  2 - Unable to determine status

This command is designed for use in scripts and automation.`,
	RunE: runUpgradeCheck,
}

func init() {
	upgradeCheckCmd.SetOut(os.Stdout)
	upgradeCheckCmd.SetErr(os.Stderr)
}

func runUpgradeCheck(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	status, err := client.GetNodeStatus(ctx)
	if err != nil {
		// Exit code 2 for unable to determine
		cmd.PrintErrln("Unable to determine node status:", err)
		os.Exit(2)
	}

	if !status.UpgradeSafe {
		// Exit code 1 for not safe
		if status.IsProving {
			cmd.PrintErrln("Not safe: node is currently proving")
		} else if status.InChallengeWindow && !status.HasProven {
			cmd.PrintErrln("Not safe: node is in an unproven challenge window")
		} else {
			cmd.PrintErrln("Not safe: node is busy")
		}
		return fmt.Errorf("upgrade not safe")
	}

	// Exit code 0 for safe
	cmd.Println("Safe to upgrade")
	return nil
}