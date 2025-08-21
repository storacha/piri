package delegate

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-libstoracha/capabilities/blob/replica"
	"github.com/storacha/go-libstoracha/capabilities/pdp"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/principal/signer"

	"github.com/storacha/piri/cmd/cliutil"
	"github.com/storacha/piri/pkg/config"
)

var (
	Cmd = &cobra.Command{
		Use:     "delegate",
		Aliases: []string{"dg"},
		Args:    cobra.NoArgs,
		Short:   `Operations for UCAN Delegations`,
	}

	GenerateCmd = &cobra.Command{
		Use:     "generate",
		Aliases: []string{"gen"},
		Args:    cobra.NoArgs,
		Short:   `Generate a new delegation`,
		RunE:    doGenerate,
	}
)

func init() {
	GenerateCmd.Flags().String(
		"client-did",
		"",
		"did of client delegation is for",
	)
	cobra.CheckErr(GenerateCmd.MarkFlagRequired("client-did"))

	GenerateCmd.Flags().String(
		"client-web-did",
		"",
		"web-did of client delegation is for, will cause delegation to wrap client did",
	)
	cobra.CheckErr(GenerateCmd.Flags().MarkHidden("client-web-did"))

	GenerateCmd.Flags().Bool(
		"car",
		false,
		"Output delegation as a Content Addressed aRchive (CAR)",
	)
	cobra.CheckErr(GenerateCmd.Flags().MarkHidden("car"))

	Cmd.AddCommand(GenerateCmd)
}

func doGenerate(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load[config.Client]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	id, err := cliutil.ReadPrivateKeyFromPEM(cfg.Identity.KeyFile)
	if err != nil {
		return fmt.Errorf("parsing private key: %w", err)
	}

	if cmd.Flags().Changed("client-web-did") {
		cwd, err := cmd.Flags().GetString("client-web-did")
		if err != nil {
			return fmt.Errorf("getting --client-web-did flag: %w", err)
		}
		if !strings.HasPrefix(cwd, "did:web:") {
			return fmt.Errorf("issuer did:web: must start with 'did:web:' prefix")
		}
		issuerDidWeb, err := did.Parse(cwd)
		if err != nil {
			return fmt.Errorf("parsing issuer did web key (%s): %w", cwd, err)
		}
		id, err = signer.Wrap(id, issuerDidWeb)
		if err != nil {
			return fmt.Errorf("wrapping issuer with did web key (%s): %w", cwd, err)
		}
	}

	didStr, err := cmd.Flags().GetString("client-did")
	if err != nil {
		return fmt.Errorf("parsing --client-did flag: %w", err)
	}
	clientDid, err := did.Parse(didStr)
	if err != nil {
		return fmt.Errorf("parsing client-did: %w", err)
	}

	d, err := MakeDelegation(
		id,        // issuer
		clientDid, // audience
		[]string{
			blob.AllocateAbility,
			blob.AcceptAbility,
			pdp.InfoAbility,
			replica.AllocateAbility,
		},
		delegation.WithNoExpiration(),
	)
	if err != nil {
		return fmt.Errorf("creating delegation: %w", err)
	}
	if cmd.Flags().Changed("car") {
		_, err := io.Copy(os.Stdout, d.Archive())
		if err != nil {
			return fmt.Errorf("writing delegation as CAR to stdout: %w", err)
		}
		return nil
	}

	out, err := FormatDelegation(d.Archive())
	if err != nil {
		return fmt.Errorf("formatting delegation as multibase-base64-encoded CIDv1: %w", err)
	}
	cmd.Println(out)
	return nil
}
