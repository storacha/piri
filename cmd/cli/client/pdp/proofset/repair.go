package proofset

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/pdp/httpapi/client"
)

var (
	RepairCmd = &cobra.Command{
		Use:   "repair",
		Short: "Repair a proof set by reconciling stuck roots with on-chain state",
		Long: `Repair a proof set by comparing on-chain state with the database.

This command fetches all active pieces from the blockchain contract and compares
them with the roots stored in the database. Any pieces that exist on-chain but
are missing from the database (due to Lotus state loss or other issues) will be
repaired using metadata from pending root additions.

Use this command when the PDP proving pipeline is stuck because roots were added
to the blockchain but never recorded in the database.`,
		Args: cobra.NoArgs,
		RunE: doRepair,
	}
)

func init() {
	RepairCmd.Flags().Uint64(
		"proofset-id",
		0,
		"The proof set ID to repair",
	)
	cobra.CheckErr(viper.BindPFlag("ucan.proof_set", RepairCmd.Flags().Lookup("proofset-id")))
}

func doRepair(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load[config.Client]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if cfg.UCAN.ProofSetID == 0 {
		return fmt.Errorf("proofset-id required, provide it with '--proofset-id' flag, or pass a valid config file")
	}

	api, err := client.NewFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	result, err := api.RepairProofSet(ctx, cfg.UCAN.ProofSetID)
	if err != nil {
		return fmt.Errorf("repairing proof set: %w", err)
	}

	jsonResult, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("rendering json: %w", err)
	}
	cmd.Print(string(jsonResult))
	return nil
}
