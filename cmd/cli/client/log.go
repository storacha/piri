package client

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/storacha/piri/pkg/admin"
)

var LogCmd = &cobra.Command{
	Use:   "log",
	Short: "Manage logging subsystems and levels",
}

func init() {
	logListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all logging subsystems and their levels",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client := admin.NewClient(viper.GetString("node_url"))

			// First get the list of subsystems from the server
			subsystems, err := client.ListLogSubsystems(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to get logging subsystems: %w", err)
			}

			// Then get the current log levels
			levels, err := client.ListLogLevels(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to get log levels: %w", err)
			}

			// Print each subsystem with its level
			for _, subsystem := range subsystems.Subsystems {
				level, exists := levels.Levels[subsystem]
				if !exists {
					level = "UNKNOWN"
				}
				cmd.Printf("%-30s %s\n", subsystem, level)
			}

			return nil
		},
	}

	logSetLevelCmd := &cobra.Command{
		Use:   "set-level <level>",
		Short: "Set log level for a subsystem or all subsystems",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			level := args[0]
			systems, err := cmd.Flags().GetStringSlice("system")
			if err != nil {
				return err
			}

			client := admin.NewClient(viper.GetString("node_url"))

			if len(systems) == 0 {
				// If no systems are specified, get all subsystems from the server
				subsystems, err := client.ListLogSubsystems(cmd.Context())
				if err != nil {
					return fmt.Errorf("failed to get logging subsystems: %w", err)
				}
				systems = subsystems.Subsystems
			}

			for _, system := range systems {
				if err := client.SetLogLevel(cmd.Context(), system, level); err != nil {
					return err
				}
			}

			return nil
		},
	}

	LogCmd.AddCommand(logListCmd)
	LogCmd.AddCommand(logSetLevelCmd)
	logSetLevelCmd.Flags().StringSlice("system", []string{}, "Subsystem to target. Pass multiple times for multiple systems.")
}
