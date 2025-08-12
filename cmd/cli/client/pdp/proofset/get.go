package proofset

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/pdp/httpapi/client"
)

var (
	GetCmd = &cobra.Command{
		Use:     "get",
		Aliases: []string{"g"},
		Short:   "Get metadata on proof set",
		Args:    cobra.NoArgs,
		RunE:    doGet,
	}
)

func init() {
	// TODO we can make this an arg instead
	GetCmd.Flags().Uint64(
		"proofset-id",
		0,
		"The proofset ID",
	)
	cobra.CheckErr(GetCmd.MarkFlagRequired("proofset-id"))
}

func doGet(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load[config.Client]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	api, err := client.NewFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	proofSetID, err := cmd.Flags().GetUint64("proofset-id")
	if err != nil {
		return fmt.Errorf("parsing proofset ID: %w", err)
	}

	proofSet, err := api.GetProofSet(ctx, proofSetID)
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
