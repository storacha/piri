package service

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/proof"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/types"
)

var PieceSizeLimit = abi.PaddedPieceSize(proof.MaxMemtreeSize).Unpadded()

func CommP(piece types.Piece, db *gorm.DB) (cid.Cid, bool, error) {
	// commp, known, error
	mh, err := Multihash(piece)
	if err != nil {
		return cid.Undef, false, err
	}

	if piece.Name == multihash.Codes[multihash.SHA2_256_TRUNC254_PADDED] {
		return cid.NewCidV1(cid.FilCommitmentUnsealed, mh), true, nil
	}

	// TODO would like to avoid using this mapping as I _think_ storacha only does the above
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

func MaybeStaticCommp(piece types.Piece) (cid.Cid, bool) {
	mh, err := Multihash(piece)
	if err != nil {
		return cid.Undef, false
	}

	if piece.Name == multihash.Codes[multihash.SHA2_256_TRUNC254_PADDED] {
		return cid.NewCidV1(cid.FilCommitmentUnsealed, mh), true
	}

	return cid.Undef, false
}
