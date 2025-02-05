package main

import (
	"os"

	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/storage/cmd"
	"github.com/urfave/cli/v2"
)

var log = logging.Logger("storage")

func main() {
	app := &cli.App{
		Name:  "storage",
		Usage: "Manage running a storage node.",
		Commands: []*cli.Command{
			cmd.StartCmd,
			cmd.IdentityCmd,
			cmd.DelegationCmd,
			cmd.ClientCmd,
			cmd.ProofSetCmd,
			cmd.VersionCmd,
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
