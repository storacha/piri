package payment

import (
	"encoding/json"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var outputFormat string

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display payment account status with interactive TUI",
	Args:  cobra.NoArgs,
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().StringVar(&outputFormat, "format", "table", "Output format: table (TUI) or json")
}

func runStatus(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	api, err := loadClient()
	if err != nil {
		return err
	}

	accountInfo, err := api.GetAccountInfo(ctx)
	if err != nil {
		return fmt.Errorf("getting account info: %w", err)
	}

	switch outputFormat {
	case "json":
		data, err := json.MarshalIndent(accountInfo, "", "  ")
		if err != nil {
			return fmt.Errorf("rendering account info: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil

	case "table":
		m := newStatusModel(accountInfo, api)
		p := tea.NewProgram(m)
		_, err := p.Run()
		return err

	default:
		return fmt.Errorf("unknown format: %s (use 'table' or 'json')", outputFormat)
	}
}
