package admin

import (
	"github.com/spf13/cobra"

	"github.com/storacha/piri/cmd/cli/client/admin/log"
)

var Cmd = &cobra.Command{
	Use:   "admin",
	Short: "Manage admin interface",
}

func init() {
	Cmd.AddCommand(log.Cmd)
}
