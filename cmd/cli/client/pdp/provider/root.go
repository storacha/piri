package provider

import (
	"github.com/spf13/cobra"
)

var (
	Cmd = &cobra.Command{
		Use:   "provider",
		Short: "Interact with PDP provider operations",
	}
)

func init() {
	Cmd.AddCommand(RegisterCmd)
	Cmd.AddCommand(StatusCmd)
}
