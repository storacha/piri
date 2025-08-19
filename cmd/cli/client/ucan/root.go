package ucan

import (
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "ucan",
	Short: "Interact with a Piri ucan server",
}

func init() {
	Cmd.AddCommand(UploadCmd)
}
