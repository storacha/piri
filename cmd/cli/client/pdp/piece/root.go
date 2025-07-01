package piece

import (
	"github.com/spf13/cobra"
)

var (
	Cmd = &cobra.Command{
		Use:   "piece",
		Short: "Interact with PDP Pieces",
	}
)

func init() {
	Cmd.AddCommand(InfoCmd)
}
