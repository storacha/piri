package proofset

import (
	"github.com/spf13/cobra"
)

var (
	Cmd = &cobra.Command{
		Use:     "proofset",
		Aliases: []string{"ps"},
		Short:   "Interact with PDP proof-set(s)",
	}
)

func init() {
	Cmd.AddCommand(CreateCmd)
	Cmd.AddCommand(StatusCmd)
	Cmd.AddCommand(GetCmd)
	Cmd.AddCommand(ListCmd)
}
