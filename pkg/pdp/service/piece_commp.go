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
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (p *PDPService) CalculateCommP(ctx context.Context, blob multihash.Multihash) (cid.Cid, error) {
	piece, err := p.pieceReader.ReadPiece(ctx, blob)
	if err != nil {
		return cid.Undef, err
	}
	defer piece.Data.Close()

	pieceCID, paddedSize, err := doCommp(blob, piece.Data, uint64(piece.Size))
	if err != nil {
		return cid.Undef, err
	}

	if err := p.db.Transaction(func(tx *gorm.DB) error {
		// 1. Create a long-term parked piece entry (marked as complete immediately).
		parkedPiece := models.ParkedPiece{
			PieceCID:        pieceCID.String(),
			PiecePaddedSize: int64(paddedSize),
			PieceRawSize:    piece.Size,
			LongTerm:        true,
			Complete:        true, // Mark as complete since it's already in PDPStore
		}
		if err := tx.Create(&parkedPiece).Error; err != nil {
			return fmt.Errorf("failed to create %s entry: %w", parkedPiece.TableName(), err)
		}

		// 2. Create a parked piece ref pointing to PDPStore.
		// NB this field is meaningless, but we might want to use the multihash for the value
		// since that's the key in the store
		dataURL := fmt.Sprintf("pdpstore://%s", blob.String())

		parkedPieceRef := models.ParkedPieceRef{
			PieceID:     parkedPiece.ID,
			DataURL:     dataURL,
			LongTerm:    true,
			DataHeaders: datatypes.JSON("{}"), // default empty JSON
		}
		if err := tx.Create(&parkedPieceRef).Error; err != nil {
			return fmt.Errorf("failed to create %s entry: %w", parkedPieceRef.TableName(), err)
		}

		// 3. insert into pdp_piece_mh_to_commp iff we derived a new CID
		if pieceCID.Hash().HexString() != blob.HexString() {
			mhToCommp := models.PDPPieceMHToCommp{
				Mhash: blob,
				Size:  piece.Size,
				Commp: pieceCID.String(),
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&mhToCommp).Error; err != nil {
				return fmt.Errorf("failed to insert into %s: %w", mhToCommp.TableName(), err)
			}
		}

		// 4. Move the entry from pdp_piece_uploads to pdp_piecerefs
		ref := models.PDPPieceRef{
			Service:  "storacha",
			PieceCID: pieceCID.String(),
			PieceRef: parkedPieceRef.RefID,
		}
		if err := tx.Create(&ref).Error; err != nil {
			return fmt.Errorf("failed to create %s entry: %w", ref.TableName(), err)
		}

		// nil returns will commit the transaction.
		return nil
	}); err != nil {
		return cid.Undef, fmt.Errorf("failed to process piece upload: %w", err)
	}

	return pieceCID, nil
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
		return pieceCID, uint64(32) << treeHeight, nil
	case uint64(multicodec.Fr32Sha256Trunc254Padbintree):
		pieceCID, err := commcid.DataCommitmentToPieceCidv2(piece.Digest, size)
		if err != nil {
			return cid.Undef, 0, err
		}
		treeHeight, _, err := commcid.PayloadSizeToV1TreeHeightAndPadding(size)
		if err != nil {
			return cid.Undef, 0, err
		}
		return pieceCID, uint64(32) << treeHeight, nil
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
