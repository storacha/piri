package proofset

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/storacha/piri/cmd/cliutil"
	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/pdp/curio"
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
		"ref-url",
		"",
		"The reference URL of a proof set, e.g. /pdp/proof-sets/created/<TX_HASH>",
	)
	cobra.CheckErr(StatusCmd.MarkFlagRequired("ref-url"))
}

func doStatus(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load[config.PDPClient]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	id, err := cliutil.ReadPrivateKeyFromPEM(cfg.KeyFile)
	if err != nil {
		return fmt.Errorf("loading key file: %w", err)
	}

	nodeAuth, err := curio.CreateCurioJWTAuthHeader("storacha", id)
	if err != nil {
		return fmt.Errorf("generating node JWT: %w", err)
	}

	nodeURL, err := url.Parse(cfg.NodeURL)
	if err != nil {
		return fmt.Errorf("parsing node URL: %w", err)
	}

	client := curio.New(http.DefaultClient, nodeURL, nodeAuth)
	refURL, err := cmd.Flags().GetString("ref-url")
	if err != nil {
		return fmt.Errorf("parsing ref URL: %w", err)
	}
	status, err := checkStatus(ctx, client, refURL)
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

func checkStatus(ctx context.Context, client curio.PDPClient, refURL string) (curio.ProofSetStatus, error) {
	status, err := client.ProofSetCreationStatus(ctx, curio.StatusRef{
		URL: refURL,
	})
	if err != nil {
		return curio.ProofSetStatus{}, fmt.Errorf("getting proof set status: %w", err)
	}
	return status, nil
}
