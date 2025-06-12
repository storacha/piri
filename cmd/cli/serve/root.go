package serve

import (
	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
)

var log = logging.Logger("cmd/serve")

var Cmd = &cobra.Command{
	Use:   "serve",
	Short: "Start a server",
}

func init() {
	Cmd.AddCommand(PDPCmd)
	Cmd.AddCommand(UCANCmd)
}
