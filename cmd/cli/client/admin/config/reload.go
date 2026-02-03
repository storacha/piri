package config

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var reloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Reload configuration from file",
	Long: `Triggers the server to re-read the config file and apply any changes.

This is useful when you've manually edited the config file and want to apply
the changes without restarting the server.

Note: This only reloads values that are set in the config file. Values that
were set via the API but not persisted will remain unchanged.`,
	Args: cobra.NoArgs,
	RunE: doReload,
}

func init() {
	Cmd.AddCommand(reloadCmd)
}

func doReload(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	api, err := loadClient()
	if err != nil {
		return err
	}

	cfg, err := api.ReloadConfig(ctx)
	if err != nil {
		return fmt.Errorf("reloading config: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Configuration reloaded from file")

	// Print reloaded values
	data, err := json.MarshalIndent(cfg.Values, "", "  ")
	if err != nil {
		return fmt.Errorf("rendering config: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))

	return nil
}
