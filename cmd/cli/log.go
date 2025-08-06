package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/storacha/piri/cmd/cliutil"
	"github.com/storacha/piri/pkg/admin"
)

func NewLogCmd() *cobra.Command {
	logCmd := &cobra.Command{
		Use:   "log",
		Short: "Manage logging subsystems and levels",
	}
	logCmd.PersistentFlags().String("manage-api", "127.0.0.1:8888", "Management API address")

	logListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all logging subsystems and their levels",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client := admin.NewClient(cliutil.MustGetManageAPI(cmd))
			resp, err := client.ListLogLevels(cmd.Context())
			if err != nil {
				return err
			}

			for subsystem, level := range resp.Levels {
				fmt.Printf("%-30s %s\n", subsystem, level)
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

			client := admin.NewClient(cliutil.MustGetManageAPI(cmd))

			if len(systems) == 0 {
				// If no systems are specified, get all of them and set the level
				resp, err := client.ListLogLevels(cmd.Context())
				if err != nil {
					return err
				}
				for subsystem := range resp.Levels {
					if err := client.SetLogLevel(cmd.Context(), subsystem, level); err != nil {
						return err
					}
				}
				return nil
			}

			for _, system := range systems {
				if err := client.SetLogLevel(cmd.Context(), system, level); err != nil {
					return err
				}
			}

			return nil
		},
	}

	logCmd.AddCommand(logListCmd)
	logCmd.AddCommand(logSetLevelCmd)
	logSetLevelCmd.Flags().StringSlice("system", []string{}, "Subsystem to target. Pass multiple times for multiple systems.")

	return logCmd
}
