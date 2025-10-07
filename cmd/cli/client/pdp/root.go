package pdp

import (
	"github.com/spf13/cobra"

	"github.com/storacha/piri/cmd/cli/client/pdp/proofset"
	"github.com/storacha/piri/cmd/cli/client/pdp/provider"
)

var Cmd = &cobra.Command{
	Use:   "pdp",
	Short: "Interact with a Piri PDP Server",
}

func init() {
	Cmd.AddCommand(proofset.Cmd)
	Cmd.AddCommand(provider.Cmd)
}
