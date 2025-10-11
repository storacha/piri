package provider

import (
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "provider",
	Short: "Manage service providers",
	Long:  `Commands for managing service providers in the FilecoinWarmStorageService contract.`,
}

func init() {
	Cmd.AddCommand(approveCmd)
	Cmd.AddCommand(listCmd)
}
