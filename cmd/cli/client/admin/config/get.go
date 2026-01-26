package config

import (
	"fmt"

	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a dynamic configuration value",
	Args:  cobra.ExactArgs(1),
	RunE:  doGet,
}

func init() {
	Cmd.AddCommand(getCmd)
}

func doGet(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	key := args[0]

	api, err := loadClient()
	if err != nil {
		return err
	}

	cfg, err := api.GetConfig(ctx)
	if err != nil {
		return fmt.Errorf("getting config: %w", err)
	}

	value, ok := cfg.Values[key]
	if !ok {
		return fmt.Errorf("key %q not found in config", key)
	}

	fmt.Fprintln(cmd.OutOrStdout(), value)
	return nil
}
