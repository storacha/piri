package proofset

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"github.com/storacha/piri/cmd/cliutil"
	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/pdp/curio"
)

var (
	CreateCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a proofset",
		Args:  cobra.NoArgs,
		RunE:  doCreate,
	}
)

func init() {
	CreateCmd.Flags().String(
		"record-keeper",
		"",
		"Hex Address of the PDP Contract Record Keeper (Service Contract)",
	)
	CreateCmd.Flags().Bool(
		"wait",
		false,
		"Poll proof set creation status, exits when proof set is created",
	)
	cobra.CheckErr(CreateCmd.MarkFlagRequired("record-keeper"))
}

func doCreate(cmd *cobra.Command, _ []string) error {
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

	recordKeeper, err := cmd.Flags().GetString("record-keeper")
	if err != nil {
		return fmt.Errorf("loading record-keeper: %w", err)
	}
	if !common.IsHexAddress(recordKeeper) {
		return fmt.Errorf("record keeper address (%s) is invalid", recordKeeper)
	}

	pdpClient := curio.New(http.DefaultClient, nodeURL, nodeAuth)
	statusRef, err := pdpClient.CreateProofSet(ctx, curio.CreateProofSet{
		RecordKeeper: recordKeeper,
	})
	if err != nil {
		return fmt.Errorf("creating proofset: %w", err)
	}
	// Write initial status to stderr
	stderr := cmd.ErrOrStderr()
	fmt.Fprintf(stderr, "Proof set being created, check status at:\n")
	fmt.Fprintf(stderr, "%s\n", statusRef.URL)

	wait, err := cmd.Flags().GetBool("wait")
	if err != nil {
		return fmt.Errorf("loading wait flag: %w", err)
	}

	if !wait {
		return nil
	}

	// Poll for status updates
	return pollProofSetStatus(ctx, pdpClient, statusRef.URL, cmd.OutOrStdout(), stderr)
}

// pollProofSetStatus polls the proof set status until creation is complete
func pollProofSetStatus(ctx context.Context, client curio.PDPClient, statusURL string, stdout, stderr io.Writer) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	spinnerChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinnerIndex := 0

	var lastStatus *curio.ProofSetStatus
	var lastOutput string

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			status, err := checkStatus(ctx, client, statusURL)
			if err != nil {
				return fmt.Errorf("checking status: %w", err)
			}

			// Generate current status output
			var output strings.Builder
			output.WriteString(fmt.Sprintf("\r%s Polling proof set status...\n", spinnerChars[spinnerIndex]))
			output.WriteString(fmt.Sprintf("  Status: %s\n", status.TxStatus))
			output.WriteString(fmt.Sprintf("  Transaction Hash: %s\n", status.CreateMessageHash))
			output.WriteString(fmt.Sprintf("  Created: %t\n", status.ProofsetCreated))
			output.WriteString(fmt.Sprintf("  Service: %s\n", status.Service))

			if status.OK != nil {
				output.WriteString(fmt.Sprintf("  Ready: %t\n", *status.OK))
			}

			if status.ProofSetId != nil {
				output.WriteString(fmt.Sprintf("  ProofSet ID: %d\n", *status.ProofSetId))
			}

			currentOutput := output.String()

			// Only update display if status changed
			if lastStatus == nil || !statusEqual(lastStatus, &status) || currentOutput != lastOutput {
				// Clear previous lines
				if lastOutput != "" {
					lines := strings.Count(lastOutput, "\n")
					fmt.Fprintf(stderr, "\033[%dA\033[K", lines) // Move up and clear lines
				}

				// Write new status
				fmt.Fprint(stderr, currentOutput)

				lastStatus = &status
				lastOutput = currentOutput
			}

			// Update spinner
			spinnerIndex = (spinnerIndex + 1) % len(spinnerChars)

			// Check if creation is complete
			if status.ProofSetId != nil {
				// Clear the status display
				lines := strings.Count(lastOutput, "\n")
				fmt.Fprintf(stderr, "\033[%dA\033[K", lines)

				// Write final status to stderr
				fmt.Fprintf(stderr, "✓ Proof set created successfully!\n")
				fmt.Fprintf(stderr, "  Transaction Hash: %s\n", status.CreateMessageHash)
				fmt.Fprintf(stderr, "  Service: %s\n", status.Service)
				fmt.Fprintf(stderr, "  ProofSet ID: %d\n", *status.ProofSetId)

				// Write only the ProofSet ID to stdout for redirection
				fmt.Fprintf(stdout, "%d\n", *status.ProofSetId)

				return nil
			}
		}
	}
}

// statusEqual compares two ProofSetStatus structs for equality
func statusEqual(a, b *curio.ProofSetStatus) bool {
	if a == nil || b == nil {
		return a == b
	}

	if a.CreateMessageHash != b.CreateMessageHash ||
		a.ProofsetCreated != b.ProofsetCreated ||
		a.Service != b.Service ||
		a.TxStatus != b.TxStatus {
		return false
	}

	// Compare OK pointers
	if (a.OK == nil) != (b.OK == nil) {
		return false
	}
	if a.OK != nil && *a.OK != *b.OK {
		return false
	}

	// Compare ProofSetId pointers
	if (a.ProofSetId == nil) != (b.ProofSetId == nil) {
		return false
	}
	if a.ProofSetId != nil && *a.ProofSetId != *b.ProofSetId {
		return false
	}

	return true
}
