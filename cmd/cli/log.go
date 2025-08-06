package cli

import (
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
	"github.com/storacha/piri/cmd/cli/client"
)

var logListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all logging subsystems and their levels",
	RunE: func(cmd *cobra.Command, args []string) error {
		// List all registered subsystems
		subsystems := logging.GetSubsystems()
		for _, subsystem := range subsystems {
			level := logging.Logger(subsystem).Level().String()
			fmt.Printf("%-30s %s\n", subsystem, level)
		}
		return nil
	},
}

var logSetLevelCmd = &cobra.Command{
	Use:   "set-level <subsystem> <level>",
	Short: "Set log level for a subsystem",
	Args: func(cmd *cobra.Command, args []string) error {
		all, err := cmd.Flags().GetBool("all")
		if err != nil {
			return err
		}
		if all {
			return cobra.ExactArgs(1)(cmd, args)
		}
		return cobra.ExactArgs(2)(cmd, args)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		all, err := cmd.Flags().GetBool("all")
		if err != nil {
			return err
		}
		if all {
			level := args[0]
			// get all subsystems
			subsystems := logging.GetSubsystems()
			for _, subsystem := range subsystems {
				if err := client.SetLogLevel(cmd.Context(), subsystem, level); err != nil {
					return err
				}
			}
			return nil
		}
		subsystem := args[0]
		level := args[1]
		return client.SetLogLevel(cmd.Context(), subsystem, level)
	},
}

func init() {
	logSetLevelCmd.Flags().Bool("all", false, "Set level for all subsystems")
	LogCmd.AddCommand(logListCmd)
	LogCmd.AddCommand(logSetLevelCmd)
}

var LogCmd = &cobra.Command{
	Use:   "log",
	Short: "Manage logging subsystems and levels",
}
