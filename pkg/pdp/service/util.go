package service

import (
	"fmt"

	commcid "github.com/filecoin-project/go-fil-commcid"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
	"github.com/storacha/piri/pkg/pdp/proof"
)

var PieceSizeLimit = abi.PaddedPieceSize(proof.MaxMemtreeSize).Unpadded()

func asPieceCIDv1(cidStr string) (cid.Cid, error) {
	pieceCid, err := cid.Decode(cidStr)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to decode PieceCID: %w", err)
	}
	if pieceCid.Prefix().MhType == uint64(multicodec.Fr32Sha256Trunc254Padbintree) {
		c1, _, err := commcid.PieceCidV1FromV2(pieceCid)
		return c1, err
	}
	return pieceCid, nil
}
