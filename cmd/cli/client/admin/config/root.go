package config

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/storacha/piri/pkg/admin/httpapi"
	"github.com/storacha/piri/pkg/admin/httpapi/client"
	"github.com/storacha/piri/pkg/config"
)

var Cmd = &cobra.Command{
	Use:   "config",
	Short: "Manage dynamic configuration",
}

var setCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a dynamic configuration value",
	Long: `Set a dynamic configuration value.
Examples:
  piri client admin config set pdp.aggregation.manager.poll_interval 1m`,
	Args: cobra.ExactArgs(2),
	RunE: doSet,
}

var persistFlag bool

func init() {
	setCmd.Flags().BoolVar(&persistFlag, "persist", false, "Persist the change to the config file")
	Cmd.AddCommand(setCmd)
}

func doSet(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	key := args[0]
	value := args[1]

	api, err := loadClient()
	if err != nil {
		return err
	}

	// Pass the value as a string - the registry will handle parsing and validation
	req := httpapi.UpdateConfigRequest{
		Updates: map[string]any{
			key: value,
		},
		Persist: persistFlag,
	}

	_, err = api.UpdateConfig(ctx, req)
	if err != nil {
		return fmt.Errorf("updating config: %w", err)
	}

	if persistFlag {
		fmt.Fprintf(cmd.OutOrStdout(), "%s updated and persisted\n", key)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "%s updated\n", key)
	}
	return nil
}

func loadClient() (*client.Client, error) {
	cfg, err := config.Load[config.Client]()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	api, err := client.NewFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating admin client: %w", err)
	}
	return api, nil
}
