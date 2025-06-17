package cmd

import (
	"github.com/urfave/cli/v2"
)

func RequiredStringFlag(strFlag *cli.StringFlag) *cli.StringFlag {
	cpy := *strFlag
	cpy.Required = true
	return &cpy
}

func RequiredIntFlag(strFlag *cli.Int64Flag) *cli.Int64Flag {
	cpy := *strFlag
	cpy.Required = true
	return &cpy
}

func RequiredUintFlag(strFlag *cli.Uint64Flag) *cli.Uint64Flag {
	cpy := *strFlag
	cpy.Required = true
	return &cpy
}

var CurioURLFlag = &cli.StringFlag{
	Name:    "pdp-server-url",
	Aliases: []string{"curio-url", "c"},
	Usage:   "URL of a running PDP server instance (formerly curio-url)",
	EnvVars: []string{"PIRI_PDP_SERVER_URL", "PIRI_CURIO_URL"},
}

var KeyFileFlag = &cli.PathFlag{
	Name:      "key-file",
	Usage:     "Path to a file containing ed25519 private key, typically created by the id gen command.",
	EnvVars:   []string{"PIRI_PRIVATE_KEY"},
	Required:  true,
	TakesFile: true,
}

var NodeDIDFlag = &cli.StringFlag{
	Name:     "node-did",
	Aliases:  []string{"nd"},
	Usage:    "did for the storage node",
	EnvVars:  []string{"PIRI_NODE_DID"},
	Required: true,
}

var NodeURLFlag = &cli.StringFlag{
	Name:     "node-url",
	Aliases:  []string{"nu"},
	Usage:    "url for the storage node",
	EnvVars:  []string{"PIRI_NODE_URL"},
	Required: true,
}

var ProofFlag = &cli.StringFlag{
	Name:     "proof",
	Aliases:  []string{"p"},
	Usage:    "CAR file containing a storage proof delegation",
	EnvVars:  []string{"PIRI_CLIENT_PROOF"},
	Required: true,
}

var ClientSetupFlags = []cli.Flag{
	KeyFileFlag,
	NodeDIDFlag,
	NodeURLFlag,
	ProofFlag,
}

var ProofSetFlag = &cli.Uint64Flag{
	Name:    "pdp-proofset",
	Aliases: []string{"pdp"},
	Usage:   "Proofset to use with PDP",
	EnvVars: []string{"PIRI_PDP_PROOFSET"},
}
