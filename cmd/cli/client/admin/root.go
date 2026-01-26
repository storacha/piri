package admin

import (
	"github.com/spf13/cobra"

	"github.com/storacha/piri/cmd/cli/client/admin/config"
	"github.com/storacha/piri/cmd/cli/client/admin/log"
	"github.com/storacha/piri/cmd/cli/client/admin/payment"
)

var Cmd = &cobra.Command{
	Use:   "admin",
	Short: "Manage admin interface",
}

func init() {
	Cmd.AddCommand(log.Cmd)
	Cmd.AddCommand(payment.Cmd)
	Cmd.AddCommand(config.Cmd)
}
