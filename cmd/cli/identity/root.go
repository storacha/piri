package identity

import (
	"bytes"
	crypto_ed25519 "crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	ed25519 "github.com/storacha/go-ucanto/principal/ed25519/signer"
)

var (
	Cmd = &cobra.Command{
		Use:     "identity",
		Aliases: []string{"id"},
		Short:   "identity commands",
		Long: `This command provides a set of subcommands for working with decentralized identities.
Specifically for generating and managing Ed25519 keys used in DID (Decentralized Identifier) systems.
  - Generate new Ed25519 key pairs encoded in PEM-format
  - Extract DID from PEM file
`,
	}

	GenerateCmd = &cobra.Command{
		Use:     "generate",
		Aliases: []string{"gen"},
		Args:    cobra.NoArgs,
		Short:   "generate an identity",
		Long: `Generate a new PEM-encoded Ed25519 key pair for use with decentralized identities (DIDs).
The command will output the key to stdout, which can be redirected to a file. 
The DID is printed to stderr for convenience.
`,
		Example: "piri identity generate > my-key.pem",
		RunE:    doGenerate,
	}
)

func init() {
	Cmd.AddCommand(GenerateCmd)
	Cmd.AddCommand(ParseCmd)
	Cmd.SetHelpFunc(identityHelpFunc())
	GenerateCmd.SetHelpFunc(identityHelpFunc())
	ParseCmd.SetHelpFunc(identityHelpFunc())
}

func doGenerate(cmd *cobra.Command, _ []string) error {
	signer, err := ed25519.Generate()
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}
	privateKey := crypto_ed25519.PrivateKey(signer.Raw())

	// Marshal and encode the private key
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("marshaling ed25519 private key: %w", err)
	}

	privateKeyBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	buffer := new(bytes.Buffer)
	if err := pem.Encode(buffer, privateKeyBlock); err != nil {
		return fmt.Errorf("encoding ed25519 private key: %w", err)
	}

	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)
	cmd.PrintErrf("# %s\n", signer.DID())
	cmd.Print(buffer.String())
	return nil
}

var ParseCmd = &cobra.Command{
	Use:     "parse",
	Short:   "parse a DID from a PEM file containing an Ed25519 key",
	Args:    cobra.ExactArgs(1),
	Example: `piri identity parse my-key.pem`,
	RunE:    doParse,
}

func doParse(cmd *cobra.Command, args []string) error {
	pemPath := args[0]
	pemFile, err := os.Open(pemPath)
	if err != nil {
		return fmt.Errorf("opening pem file: %w", err)
	}
	pemData, err := io.ReadAll(pemFile)
	if err != nil {
		return fmt.Errorf("reading pem file: %w", err)
	}

	blk, _ := pem.Decode(pemData)
	if blk == nil {
		return fmt.Errorf("no PEM block found")
	}

	sk, err := x509.ParsePKCS8PrivateKey(blk.Bytes)
	if err != nil {
		return fmt.Errorf("parsing PKCS#8 private key: %w", err)
	}

	ed25519SK, ok := sk.(crypto_ed25519.PrivateKey)
	if !ok {
		return fmt.Errorf("PKCS#8 private key does not implement ed25519")
	}

	key, err := ed25519.FromRaw(ed25519SK)
	if err != nil {
		return fmt.Errorf("decoding ed25519 private key: %w", err)
	}
	cmd.Printf("# %s\n", key.DID().String())
	return nil
}

func identityHelpFunc() func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
		fmt.Printf("Usage:\n  %s\n\n", cmd.UseLine())
		fmt.Printf("%s\n", cmd.Short)
		if cmd.Long != "" {
			fmt.Printf("\n%s\n", cmd.Long)
		}

		// Show available subcommands
		if cmd.HasAvailableSubCommands() {
			fmt.Printf("\nAvailable Commands:\n")
			for _, c := range cmd.Commands() {
				if c.IsAvailableCommand() {
					fmt.Printf("  %-15s %s\n", c.Name(), c.Short)
				}
			}
		}
		// Only show local flags
		if cmd.HasLocalFlags() {
			fmt.Printf("\nFlags:\n")
			fmt.Print(cmd.LocalFlags().FlagUsages())
		}
	}
}
