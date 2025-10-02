package provider

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/pdp/httpapi/client"
	"github.com/storacha/piri/pkg/pdp/types"
)

var (
	RegisterCmd = &cobra.Command{
		Use:   "register",
		Short: "Register as a service provider",
		Args:  cobra.NoArgs,
		RunE:  doRegister,
	}
)

func init() {
	RegisterCmd.Flags().String(
		"name",
		"",
		"Provider name (optional, max 128 chars)",
	)

	RegisterCmd.Flags().String(
		"description",
		"",
		"Provider description (optional, max 256 chars)",
	)
	cobra.CheckErr(RegisterCmd.MarkFlagRequired("name"))
	cobra.CheckErr(RegisterCmd.MarkFlagRequired("description"))
}

func doRegister(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load[config.Client]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	pdpClient, err := client.NewFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("creating pdp client: %w", err)
	}

	name, err := cmd.Flags().GetString("name")
	if err != nil {
		return fmt.Errorf("loading name flag: %w", err)
	}

	description, err := cmd.Flags().GetString("description")
	if err != nil {
		return fmt.Errorf("loading description flag: %w", err)
	}

	result, err := pdpClient.RegisterProvider(ctx, types.RegisterProviderParams{
		Name:        name,
		Description: description,
	})
	if err != nil {
		return fmt.Errorf("registering provider: %w", err)
	}

	jsonResult, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("rendering json: %w", err)
	}

	cmd.Print(string(jsonResult))
	return nil
}
