package proofset

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/pdp/httpapi/client"
	"github.com/storacha/piri/pkg/pdp/types"
)

var (
	StatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Check on progress of proofset creation",
		Args:  cobra.NoArgs,
		RunE:  doStatus,
	}
)

func init() {
	StatusCmd.Flags().String(
		"txhash",
		"",
		"The transaction hash resulting from a proof set create message",
	)
	cobra.CheckErr(StatusCmd.MarkFlagRequired("txhash"))
}

func doStatus(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load[config.PDPClient]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	api, err := client.NewFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	txhash, err := cmd.Flags().GetString("txhash")
	if err != nil {
		return fmt.Errorf("parsing txHash: %w", err)
	}
	status, err := checkStatus(ctx, api, common.HexToHash(txhash))
	if err != nil {
		return fmt.Errorf("getting proof set status: %w", err)
	}
	jsonStatus, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("rendering json: %w", err)
	}
	cmd.Print(string(jsonStatus))
	return nil
}

func checkStatus(ctx context.Context, client types.ProofSetAPI, txHash common.Hash) (*types.ProofSetStatus, error) {
	status, err := client.GetProofSetStatus(ctx, txHash)
	if err != nil {
		return nil, err
	}
	return status, nil
}
