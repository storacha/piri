package service

import (
	"context"
	"fmt"
	"io"

	commcid "github.com/filecoin-project/go-fil-commcid"
	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/types"
	"gorm.io/gorm/clause"
)

func (p *PDPService) CalculateCommP(ctx context.Context, blob multihash.Multihash) (types.CalculateCommPResponse, error) {
	key := blob.String()

	// use singleflight to prevent duplicate commp calculations
	v, err, _ := p.commPGroup.Do(key, func() (interface{}, error) {
		// 1. check if we have already calculated commp for this piece
		var existing models.PDPPieceMHToCommp
		if err := p.db.First(&existing, "mhash = ?", blob).Error; err == nil {
			pieceCID, err := cid.Parse(existing.Commp)
			if err != nil {
				return types.CalculateCommPResponse{}, fmt.Errorf("failed to parse existing commp cid %s: %w", existing.Commp, err)
			}
			treeHeight, _, err := commcid.PayloadSizeToV1TreeHeightAndPadding(uint64(existing.Size))
			if err != nil {
				return types.CalculateCommPResponse{}, err
			}
			return types.CalculateCommPResponse{
				PieceCID:   pieceCID,
				RawSize:    int64(existing.Size),
				PaddedSize: int64(32) << treeHeight,
			}, nil
		}
		// 2. calculate commp since we don't have it yet
		readObj, err := p.pieceReader.Read(ctx, blob)
		if err != nil {
			return types.CalculateCommPResponse{}, err
		}
		defer readObj.Data.Close()

		pieceCID, paddedSize, err := doCommp(blob, readObj.Data, uint64(readObj.Size))
		if err != nil {
			return types.CalculateCommPResponse{}, err
		}

		// 3. insert into pdp_piece_mh_to_commp to avoid recalculation
		if pieceCID.Hash().HexString() != blob.HexString() {
			mhToCommp := models.PDPPieceMHToCommp{
				Mhash: blob,
				Size:  int64(readObj.Size),
				Commp: pieceCID.String(),
			}
			if err := p.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&mhToCommp).Error; err != nil {
				return types.CalculateCommPResponse{}, fmt.Errorf("failed to insert into %s: %w", mhToCommp.TableName(), err)
			}
		}

		return types.CalculateCommPResponse{
			PieceCID:   pieceCID,
			RawSize:    readObj.Size,
			PaddedSize: int64(paddedSize),
		}, nil
	})

	if err != nil {
		return types.CalculateCommPResponse{}, err
	}

	return v.(types.CalculateCommPResponse), nil
}

func doCommp(blob multihash.Multihash, data io.Reader, size uint64) (cid.Cid, uint64, error) {
	piece, err := multihash.Decode(blob)
	if err != nil {
		return cid.Undef, 0, fmt.Errorf("invalid multihash: %w", err)
	}
	switch piece.Code {
	case uint64(multicodec.Sha2_256Trunc254Padded):
		// we have a pieceCID v1, convert to v2 and return padded size
		pv1, err := commcid.DataCommitmentV1ToCID(piece.Digest)
		if err != nil {
			return cid.Undef, 0, err
		}
		pieceCID, err := commcid.PieceCidV2FromV1(pv1, size)
		if err != nil {
			return cid.Undef, 0, fmt.Errorf("failed to convert pieceCid %s from v1 to v2: %w", pv1, err)
		}
		treeHeight, _, err := commcid.PayloadSizeToV1TreeHeightAndPadding(size)
		if err != nil {
			return cid.Undef, 0, err
		}
		paddedSize := uint64(32) << treeHeight
		return pieceCID, paddedSize, nil
	case uint64(multicodec.Fr32Sha256Trunc254Padbintree):
		pieceCID, err := commcid.DataCommitmentToPieceCidv2(piece.Digest, size)
		if err != nil {
			return cid.Undef, 0, err
		}
		treeHeight, _, err := commcid.PayloadSizeToV1TreeHeightAndPadding(size)
		if err != nil {
			return cid.Undef, 0, err
		}
		paddedSize := uint64(32) << treeHeight
		return pieceCID, paddedSize, nil
	default:
		// need to calculate commp
		cp := &commp.Calc{}
		written, err := io.Copy(cp, data)
		if err != nil {
			return cid.Undef, 0, err
		}

		if uint64(written) != size {
			return cid.Undef, 0, fmt.Errorf("failed to compute commp digest: expected %d bytes, got %d", size, written)
		}

		commpDigest, commpPaddedSize, err := cp.Digest()
		if err != nil {
			return cid.Undef, 0, fmt.Errorf("failed to compute commp digest: %w", err)
		}

		pieceCID, err := commcid.DataCommitmentToPieceCidv2(commpDigest, size)
		if err != nil {
			return cid.Undef, 0, fmt.Errorf("failed to convert commp digeat to piece cid v2: %w", err)
		}

		treeHeight, _, err := commcid.PayloadSizeToV1TreeHeightAndPadding(size)
		if err != nil {
			return cid.Undef, 0, err
		}

		expectedPaddedSize := uint64(32) << treeHeight
		if commpPaddedSize != expectedPaddedSize {
			return cid.Undef, 0, fmt.Errorf("unexpected padded size from commp calculation: got %d, expected %d (raw size: %d)", commpPaddedSize, expectedPaddedSize, size)
		}

		return pieceCID, commpPaddedSize, nil
	}

}
