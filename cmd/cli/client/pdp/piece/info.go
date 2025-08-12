package piece

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/spf13/cobra"
	"github.com/storacha/go-libstoracha/piece/piece"

	"github.com/storacha/piri/cmd/client"
	"github.com/storacha/piri/pkg/config"
)

var ErrMustBePieceLinkOrHaveSize = errors.New("passing pieceCID v1 requires a size to be present")

var (
	InfoCmd = &cobra.Command{
		Use:   "info",
		Short: "Get pdp piece information",
		Args:  cobra.NoArgs,
		RunE:  doInfo,
	}
)

func init() {
	InfoCmd.Flags().String("piece", "", "PieceCID to get information on")
	cobra.CheckErr(InfoCmd.MarkFlagRequired("piece"))

	InfoCmd.Flags().Uint64("size", 0, "Optional size if passing a piece cid v1 of data")
}

func doInfo(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load[config.Client]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	c, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}
	pieceStr, err := cmd.Flags().GetString("piece")
	if err != nil {
		return fmt.Errorf("getting piece from --piece flag: %w", err)
	}
	pieceCid, err := cid.Decode(pieceStr)
	if err != nil {
		return fmt.Errorf("decoding cid: %w", err)
	}
	pieceLink, err := piece.FromLink(cidlink.Link{Cid: pieceCid})
	if err != nil {
		if !cmd.Flags().Changed("size") {
			return ErrMustBePieceLinkOrHaveSize
		}
		size, err := cmd.Flags().GetUint64("size")
		if err != nil {
			return fmt.Errorf("getting size flag: %w", err)
		}
		pieceLink, err = piece.FromV1LinkAndSize(cidlink.Link{Cid: pieceCid}, size)
		if err != nil {
			return fmt.Errorf("parsing as pieceCID v1: %w", err)
		}
	}
	ok, err := c.PDPInfo(cmd.Context(), pieceLink)
	if err != nil {
		return fmt.Errorf("getting pdp info: %w", err)
	}
	asJSON, err := json.MarshalIndent(ok, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling info to json: %w", err)
	}
	cmd.Print(string(asJSON))
	return nil
}
