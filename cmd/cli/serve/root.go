package serve

import (
	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var log = logging.Logger("cmd/serve")

var Cmd = &cobra.Command{
	Use:   "serve",
	Short: "Start a server",
	Args:  cobra.NoArgs,
	RunE:  fullServer,
}

func init() {
	Cmd.AddCommand(UCANCmd)
	Cmd.AddCommand(FullCmd)

	Cmd.PersistentFlags().String(
		"host",
		"localhost",
		"Host to listen on")
	cobra.CheckErr(viper.BindPFlag("server.host", Cmd.PersistentFlags().Lookup("host")))

	Cmd.PersistentFlags().Uint(
		"port",
		3000,
		"Port to listen on",
	)
	cobra.CheckErr(viper.BindPFlag("server.port", Cmd.PersistentFlags().Lookup("port")))

	Cmd.PersistentFlags().String(
		"public-url",
		"",
		"URL the node is publicly accessible at and exposed to other storacha services",
	)
	cobra.CheckErr(viper.BindPFlag("server.public_url", Cmd.PersistentFlags().Lookup("public-url")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("server.public_url", "PIRI_PUBLIC_URL"))

}
