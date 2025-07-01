package task

import (
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "task",
	Short: "Interact with a Piri PDP task",
}

func init() {
	Cmd.AddCommand(HistoryCmd)
}
