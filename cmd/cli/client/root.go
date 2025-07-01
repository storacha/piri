package client

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/storacha/piri/cmd/cli/client/pdp"
	"github.com/storacha/piri/cmd/cli/client/ucan"
	"github.com/storacha/piri/pkg/config"
)

var (
	Cmd = &cobra.Command{
		Use:   "client",
		Short: "Interact with a Piri node",
	}
)

func init() {
	Cmd.PersistentFlags().String("node-url", config.DefaultPDPClient.NodeURL, "URL of a Piri node")
	cobra.CheckErr(viper.BindPFlag("node_url", Cmd.PersistentFlags().Lookup("node-url")))

	Cmd.AddCommand(ucan.Cmd)
	Cmd.AddCommand(pdp.Cmd)
}
