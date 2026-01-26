package config

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all dynamic configuration values",
	Args:  cobra.NoArgs,
	RunE:  doList,
}

func init() {
	Cmd.AddCommand(listCmd)
}

func doList(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	api, err := loadClient()
	if err != nil {
		return err
	}

	cfg, err := api.GetConfig(ctx)
	if err != nil {
		return fmt.Errorf("getting config: %w", err)
	}

	data, err := json.MarshalIndent(cfg.Values, "", "  ")
	if err != nil {
		return fmt.Errorf("rendering config: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}
