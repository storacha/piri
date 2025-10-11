package payments

import (
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "payments",
	Short: "Manage payments contract interactions",
	Long:  `Commands for managing operator approvals and deposits with the Payments contract.`,
}

func init() {
	Cmd.AddCommand(calculateCmd)
	Cmd.AddCommand(depositCmd)
	Cmd.AddCommand(approveServiceCmd)
	Cmd.AddCommand(statusCmd)
	Cmd.AddCommand(balanceCmd)
	Cmd.AddCommand(accountCmd)
	Cmd.AddCommand(settleCmd)
	Cmd.AddCommand(withdrawCmd)
}
