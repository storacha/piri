package service

import (
	"encoding/hex"
	"errors"
	"fmt"

	commcid "github.com/filecoin-project/go-fil-commcid"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/proof"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/types"
)

var PieceSizeLimit = abi.PaddedPieceSize(proof.MaxMemtreeSize).Unpadded()

// CommP accepts a types.Piece and returns it as a PieceCIDV2 CID.
// CommP returns true if the types.Piece exists in the PDPPieceMHToCommp table, this is useful
// for determining if the piece has already been added to the node.
func CommP(piece types.Piece, db *gorm.DB) (cid.Cid, bool, error) {
	mh, err := Multihash(piece)
	if err != nil {
		return cid.Undef, false, err
	}

	dmh, err := multihash.Decode(mh)
	if err != nil {
		return cid.Undef, false, err
	}

	switch dmh.Code {
	case uint64(multicodec.Sha2_256Trunc254Padded): // PieceCIDV1
		v1 := cid.NewCidV1(cid.FilCommitmentUnsealed, mh)
		v2, err := commcid.PieceCidV2FromV1(v1, uint64(piece.Size))
		if err != nil {
			return cid.Undef, false, err
		}
		return v2, false, nil
	case uint64(multicodec.Fr32Sha256Trunc254Padbintree): // PieceCIDV2
		v2 := cid.NewCidV1(dmh.Code, mh)
		return v2, false, nil
	default:
	}

	// the piece we were given isn't using commp, so we look up its corresponding commp (pieceCIDV2) and return it,
	//or fail if we don't have the mapping
	var record models.PDPPieceMHToCommp
	if err := db.
		Where("mhash = ? AND size = ?", mh, piece.Size).
		First(&record).
		Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			// No matching record
			return cid.Undef, false, nil
		}
		return cid.Undef, false, fmt.Errorf("failed to query pdp_piece_mh_to_commp: %w", err)
	}

	commpCid, err := cid.Parse(record.Commp)
	if err != nil {
		return cid.Undef, false, fmt.Errorf("failed to parse commp CID: %w", err)
	}

	return commpCid, true, nil

}

func Multihash(piece types.Piece) (multihash.Multihash, error) {
	_, ok := multihash.Names[piece.Name]
	if !ok {
		return nil, types.NewErrorf(types.KindInvalidInput, "unknown multihash type: %s", piece.Name)
	}

	hashBytes, err := hex.DecodeString(piece.Hash)
	if err != nil {
		return nil, types.WrapError(types.KindInvalidInput, "failed to decode hash", err)
	}

	return multihash.EncodeName(hashBytes, piece.Name)
}

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

// asPieceCIDv2 converts a string to a PieceCIDv2. Where the input is expected to be a PieceCIDv1,
// a size argument is required. Where it's expected to be a v2, the size argument is ignored. The
// size either derived from the v2 or from the size argument in the case of a v1 is returned.
func asPieceCIDv2(cidStr string, size uint64) (cid.Cid, uint64, error) {
	pieceCid, err := cid.Decode(cidStr)
	if err != nil {
		return cid.Undef, 0, fmt.Errorf("failed to decode subPieceCid: %w", err)
	}
	switch pieceCid.Prefix().MhType {
	case uint64(multicodec.Sha2_256Trunc254Padded):
		if size == 0 {
			return cid.Undef, 0, fmt.Errorf("size must be provided for PieceCIDv1")
		}
		c, err := commcid.PieceCidV2FromV1(pieceCid, size)
		if err != nil {
			return cid.Undef, 0, err
		}
		return c, size, nil
	case uint64(multicodec.Fr32Sha256Trunc254Padbintree):
		// get the size from the CID, not the argument
		_, size, err := commcid.PieceCidV2ToDataCommitment(pieceCid)
		if err != nil {
			return cid.Undef, 0, err
		}
		return pieceCid, size, nil
	default:
		return cid.Undef, 0, fmt.Errorf("unsupported piece CID type: %d", pieceCid.Prefix().MhType)
	}
}
