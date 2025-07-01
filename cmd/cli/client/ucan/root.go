package ucan

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Cmd = &cobra.Command{
	Use:   "ucan",
	Short: "Interact with a Piri ucan server",
}

func init() {
	Cmd.PersistentFlags().String("node-did", "", "DID of a Piri node")
	cobra.CheckErr(viper.BindPFlag("node_did", Cmd.PersistentFlags().Lookup("node-did")))
	cobra.CheckErr(Cmd.MarkPersistentFlagRequired("node-did"))

	Cmd.AddCommand(UploadCmd)
}
