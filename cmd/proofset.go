package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/storacha/piri/pkg/pdp/curio"
)

var ProofSetCmd = &cli.Command{
	Name:    "proofset",
	Aliases: []string{"ps"},
	Usage:   "proofset tools.",
	Subcommands: []*cli.Command{
		{
			Name:    "create",
			Aliases: []string{"c"},
			Usage:   "Generate a new proofset",
			Flags: []cli.Flag{
				KeyFileFlag,
				RequiredStringFlag(CurioURLFlag),
				&cli.StringFlag{
					Name:     "record-keeper",
					Aliases:  []string{"rk"},
					Usage:    "Hex address of the record keeper",
					EnvVars:  []string{"PIRI_RECORD_KEEPER_CONTRACT"},
					Required: true,
				},
			},
			Action: func(cCtx *cli.Context) error {
				id, err := PrincipalSignerFromFile(cCtx.String("key-file"))
				if err != nil {
					return fmt.Errorf("parsing private key: %w", err)
				}

				curioURL, err := url.Parse(cCtx.String("pdp-server-url"))
				if err != nil {
					return fmt.Errorf("parsing pdp server URL: %w", err)
				}

				curioAuth, err := curio.CreateCurioJWTAuthHeader("storacha", id)
				if err != nil {
					return fmt.Errorf("generating curio jwt: %w", err)
				}

				client := curio.New(http.DefaultClient, curioURL, curioAuth)
				statusRef, err := client.CreateProofSet(cCtx.Context, curio.CreateProofSet{
					RecordKeeper: cCtx.String("record-keeper"),
				})
				if err != nil {
					return fmt.Errorf("creating proof set: %w", err)
				}
				fmt.Printf("proof set being created, check status at %s\n", filepath.Join(curioURL.String(), statusRef.URL))
				return nil
			},
		},
		{
			Name:    "status",
			Aliases: []string{"cs"},
			Usage:   "check on progress creating a proofset",
			Flags: []cli.Flag{
				KeyFileFlag,
				RequiredStringFlag(CurioURLFlag),
				&cli.StringFlag{
					Name:     "ref-url",
					Aliases:  []string{"ru"},
					Usage:    "Ref URL from create command",
					EnvVars:  []string{"PIRI_REF_URL"},
					Required: true,
				},
			},
			Action: func(cCtx *cli.Context) error {
				curioURL, err := url.Parse(cCtx.String("curio-url"))
				if err != nil {
					return fmt.Errorf("parsing curio URL: %w", err)
				}

				id, err := PrincipalSignerFromFile(cCtx.String("key-file"))
				if err != nil {
					return fmt.Errorf("parsing private key: %w", err)
				}

				curioAuth, err := curio.CreateCurioJWTAuthHeader("storacha", id)
				if err != nil {
					return fmt.Errorf("generating curio jwt: %w", err)
				}

				client := curio.New(http.DefaultClient, curioURL, curioAuth)
				status, err := client.ProofSetCreationStatus(cCtx.Context, curio.StatusRef{
					URL: cCtx.String("ref-url"),
				})
				if err != nil {
					return fmt.Errorf("getting proof set status: %w", err)
				}
				jsonStatus, err := json.MarshalIndent(status, "", "  ")
				if err != nil {
					return fmt.Errorf("rendering json: %w", err)
				}
				fmt.Print(string(jsonStatus))
				return nil
			},
		},
		{
			Name:    "get",
			Aliases: []string{"g"},
			Usage:   "get a proofs set",
			Flags: []cli.Flag{
				KeyFileFlag,
				RequiredStringFlag(CurioURLFlag),
				RequiredUintFlag(ProofSetFlag),
			},
			Action: func(cCtx *cli.Context) error {
				curioURL, err := url.Parse(cCtx.String("curio-url"))
				if err != nil {
					return fmt.Errorf("parsing curio URL: %w", err)
				}

				id, err := PrincipalSignerFromFile(cCtx.String("key-file"))
				if err != nil {
					return fmt.Errorf("parsing private key: %w", err)
				}

				curioAuth, err := curio.CreateCurioJWTAuthHeader("storacha", id)
				if err != nil {
					return fmt.Errorf("generating curio jwt: %w", err)
				}

				client := curio.New(http.DefaultClient, curioURL, curioAuth)
				proofSet, err := client.GetProofSet(cCtx.Context, cCtx.Uint64("pdp-proofset"))
				if err != nil {
					return fmt.Errorf("getting proof set status: %w", err)
				}
				jsonProofSet, err := json.MarshalIndent(proofSet, "", "  ")
				if err != nil {
					return fmt.Errorf("rendering json: %w", err)
				}
				fmt.Print(string(jsonProofSet))
				return nil
			},
		},
	},
}
