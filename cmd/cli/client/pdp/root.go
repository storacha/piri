package pdp

import (
	"github.com/spf13/cobra"

	"github.com/storacha/piri/cmd/cli/client/pdp/piece"
	"github.com/storacha/piri/cmd/cli/client/pdp/proofset"
)

var Cmd = &cobra.Command{
	Use:   "pdp",
	Short: "Interact with a Piri PDP Server",
}

func init() {
	Cmd.AddCommand(piece.Cmd)
	Cmd.AddCommand(proofset.Cmd)
}
