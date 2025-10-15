package provider

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/pdp/httpapi/client"
)

var (
	StatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Get provider registration status",
		Args:  cobra.NoArgs,
		RunE:  doStatus,
	}
)

func doStatus(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load[config.Client]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	pdpClient, err := client.NewFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("creating pdp client: %w", err)
	}

	result, err := pdpClient.GetProviderStatus(ctx)
	if err != nil {
		return fmt.Errorf("getting provider status: %w", err)
	}

	jsonResult, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("rendering json: %w", err)
	}

	cmd.Print(string(jsonResult))
	return nil
}
