package proofset

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/pdp/httpapi/client"
)

var (
	ListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all proof sets",
		Args:  cobra.NoArgs,
		RunE:  doList,
	}
)

func doList(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load[config.Client]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	api, err := client.NewFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	proofSet, err := api.ListProofSet(ctx)
	if err != nil {
		return fmt.Errorf("getting proof set status: %w", err)
	}
	jsonProofSet, err := json.MarshalIndent(proofSet, "", "  ")
	if err != nil {
		return fmt.Errorf("rendering json: %w", err)
	}
	fmt.Print(string(jsonProofSet))
	return nil

}
