package payment

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/storacha/piri/pkg/admin/httpapi/client"
	"github.com/storacha/piri/pkg/config"
)

var Cmd = &cobra.Command{
	Use:   "payment",
	Short: "Manage payment account",
}

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Get payment account information",
	Args:  cobra.NoArgs,
	RunE:  doAccount,
}

func init() {
	Cmd.AddCommand(accountCmd)
	Cmd.AddCommand(statusCmd)
}

func doAccount(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	api, err := loadClient()
	if err != nil {
		return err
	}

	info, err := api.GetAccountInfo(ctx)
	if err != nil {
		return fmt.Errorf("getting account info: %w", err)
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("rendering account info: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
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
