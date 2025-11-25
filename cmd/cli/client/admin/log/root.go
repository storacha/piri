package log

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/storacha/piri/pkg/admin/httpapi/client"
	"github.com/storacha/piri/pkg/config"
)

var Cmd = &cobra.Command{
	Use:   "log",
	Short: "Manage log systems",
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List log systems",
	Args:  cobra.NoArgs,
	RunE:  doList,
}

var setCmd = &cobra.Command{
	Use:   "set <level> [system]",
	Short: "Set a log level for one system or all systems",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  doSet,
}

var setRegex = &cobra.Command{
	Use:   "set-regex <level> <expression>",
	Short: "Set log level for subsystems matching a regex",
	Args:  cobra.ExactArgs(2),
	RunE:  doSetRegex,
}

func init() {
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(setCmd)
	Cmd.AddCommand(setRegex)
}

func doList(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	api, err := loadClient()
	if err != nil {
		return err
	}

	loggers, err := api.ListLogLevels(ctx)
	if err != nil {
		return fmt.Errorf("listing log levels: %w", err)
	}

	data, err := json.MarshalIndent(loggers, "", "  ")
	if err != nil {
		return fmt.Errorf("rendering log levels: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

func doSet(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	level := args[0]
	var system string
	if len(args) == 2 {
		system = args[1]
	}

	api, err := loadClient()
	if err != nil {
		return err
	}

	if system == "" {
		if err := api.SetLogLevelRegex(ctx, ".*", level); err != nil {
			return fmt.Errorf("setting log level for all: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "log level updated for all systems")
		return nil
	}

	if err := api.SetLogLevel(ctx, system, level); err != nil {
		return fmt.Errorf("setting log level: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "log level updated")
	return nil
}

func doSetRegex(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	level := args[0]
	expr := args[1]

	api, err := loadClient()
	if err != nil {
		return err
	}

	if err := api.SetLogLevelRegex(ctx, expr, level); err != nil {
		return fmt.Errorf("setting log level by regex: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "log levels updated")
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
