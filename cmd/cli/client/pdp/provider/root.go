package provider

import (
	"github.com/spf13/cobra"
)

var (
	Cmd = &cobra.Command{
		Use:   "provider",
		Short: "Interact with PDP provider operations",
		// NB(forrest): this command is hidden since the intention is to register via init
		Hidden: true,
	}
)

func init() {
	Cmd.AddCommand(RegisterCmd)
	Cmd.AddCommand(StatusCmd)
}
