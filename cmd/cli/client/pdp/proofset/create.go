package proofset

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/pdp/httpapi/client"
	"github.com/storacha/piri/pkg/pdp/types"
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
	cobra.CheckErr(CreateCmd.MarkFlagRequired("record-keeper"))

	CreateCmd.Flags().Bool(
		"wait",
		false,
		"Poll proof set creation status, exits when proof set is created",
	)
}

func doCreate(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load[config.Client]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	pdpClient, err := client.NewFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("creating pdp client: %w", err)
	}

	recordKeeper, err := cmd.Flags().GetString("record-keeper")
	if err != nil {
		return fmt.Errorf("loading record-keeper: %w", err)
	}
	if !common.IsHexAddress(recordKeeper) {
		return fmt.Errorf("record keeper address (%s) is invalid", recordKeeper)
	}

	txHash, err := pdpClient.CreateProofSet(ctx, common.HexToAddress(recordKeeper))
	if err != nil {
		return fmt.Errorf("creating proofset: %w", err)
	}
	// Write initial status to stderr
	stderr := cmd.ErrOrStderr()
	fmt.Fprintf(stderr, "Proof set being created, transaction hash:\n")
	fmt.Fprintf(stderr, "%s\n", txHash.String())

	wait, err := cmd.Flags().GetBool("wait")
	if err != nil {
		return fmt.Errorf("loading wait flag: %w", err)
	}

	if !wait {
		return nil
	}

	// Poll for status updates
	return pollProofSetStatus(ctx, pdpClient, txHash, cmd.OutOrStdout(), stderr)
}

// pollProofSetStatus polls the proof set status until creation is complete
func pollProofSetStatus(ctx context.Context, client types.ProofSetAPI, txHash common.Hash, stdout, stderr io.Writer) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	spinnerChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinnerIndex := 0

	var lastStatus *types.ProofSetStatus
	var lastOutput string

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			status, err := checkStatus(ctx, client, txHash)
			if err != nil {
				return fmt.Errorf("checking status: %w", err)
			}

			// Generate current status output
			var output strings.Builder
			output.WriteString(fmt.Sprintf("\r%s Polling proof set status...\n", spinnerChars[spinnerIndex]))
			output.WriteString(fmt.Sprintf("  Status: %s\n", status.TxStatus))
			output.WriteString(fmt.Sprintf("  Transaction Hash: %s\n", status.TxHash))
			output.WriteString(fmt.Sprintf("  Created: %t\n", status.Created))

			if status.ID != 0 {
				output.WriteString(fmt.Sprintf("  ProofSet ID: %d\n", status.ID))
			}

			currentOutput := output.String()

			// Only update display if status changed
			if lastStatus == nil || !statusEqual(lastStatus, status) || currentOutput != lastOutput {
				// Clear previous lines
				if lastOutput != "" {
					lines := strings.Count(lastOutput, "\n")
					fmt.Fprintf(stderr, "\033[%dA\033[K", lines) // Move up and clear lines
				}

				// Write new status
				fmt.Fprint(stderr, currentOutput)

				lastStatus = status
				lastOutput = currentOutput
			}

			// Update spinner
			spinnerIndex = (spinnerIndex + 1) % len(spinnerChars)

			// Check if creation is complete
			if status.Created {
				// Clear the status display
				lines := strings.Count(lastOutput, "\n")
				fmt.Fprintf(stderr, "\033[%dA\033[K", lines)

				// Write final status to stderr
				fmt.Fprintf(stderr, "✓ Proof set created successfully!\n")
				fmt.Fprintf(stderr, "  Transaction Hash: %s\n", status.TxHash)
				fmt.Fprintf(stderr, "  ProofSet ID: %d\n", status.ID)
				time.Sleep(time.Second)

				// Write only the ProofSet ID to stdout for redirection
				fmt.Fprintf(stdout, "%d\n", status.ID)

				return nil
			}
		}
	}
}

// statusEqual compares two ProofSetStatus structs for equality
func statusEqual(a, b *types.ProofSetStatus) bool {
	if a == nil || b == nil {
		return a == b
	}

	if a.TxHash != b.TxHash ||
		a.Created != b.Created ||
		a.TxStatus != b.TxStatus {
		return false
	}

	// Compare ProofSetId pointers
	if (a.ID == 0) != (b.ID == 0) {
		return false
	}
	if a.ID != 0 && a.ID != b.ID {
		return false
	}

	return true
}
