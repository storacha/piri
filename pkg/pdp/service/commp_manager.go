package service

import (
	"context"
	"fmt"
	"io"

	commcid "github.com/filecoin-project/go-fil-commcid"
	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (p *PDPService) CalculateCommP(ctx context.Context, blob cid.Cid) (cid.Cid, error) {
	data, err := p.blobstore.Get(ctx, blob.Hash())
	if err != nil {
		return cid.Undef, err
	}

	dataReader := data.Body()
	defer dataReader.Close()

	pieceCID, paddedSize, err := doCommp(blob, dataReader, uint64(data.Size()))
	if err != nil {
		return cid.Undef, err
	}

	if err := p.db.Transaction(func(tx *gorm.DB) error {
		// 1. Create a long-term parked piece entry (marked as complete immediately).
		parkedPiece := models.ParkedPiece{
			PieceCID:        pieceCID.String(),
			PiecePaddedSize: int64(paddedSize),
			PieceRawSize:    data.Size(),
			LongTerm:        true,
			Complete:        true, // Mark as complete since it's already in PDPStore
		}
		if err := tx.Create(&parkedPiece).Error; err != nil {
			return fmt.Errorf("failed to create %s entry: %w", parkedPiece.TableName(), err)
		}

		// 2. Create a parked piece ref pointing to PDPStore.
		dataURL := fmt.Sprintf("pdpstore://%s", pieceCID.String())

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
		if !pieceCID.Equals(blob) {
			mhToCommp := models.PDPPieceMHToCommp{
				Mhash: blob.Hash(),
				Size:  data.Size(),
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

func doCommp(c cid.Cid, data io.Reader, size uint64) (cid.Cid, uint64, error) {
	switch c.Prefix().MhType {
	case uint64(multicodec.Sha2_256Trunc254Padded):
		// we have a pieceCID v1, convert to v2 and return padded size
		pieceCID, err := commcid.PieceCidV2FromV1(c, size)
		if err != nil {
			return cid.Undef, 0, fmt.Errorf("failed to convert pieceCid %s from v1 to v2: %w", c, err)
		}
		treeHeight, _, err := commcid.PayloadSizeToV1TreeHeightAndPadding(size)
		if err != nil {
			return cid.Undef, 0, err
		}
		return pieceCID, uint64(32) << treeHeight, nil
	case uint64(multicodec.Fr32Sha256Trunc254Padbintree):
		// we have a pieceCID v2
		// TODO we can probably skip this and return the cid with derived padded size
		digest, extractedSize, err := commcid.PieceCidV2ToDataCommitment(c)
		if err != nil {
			return cid.Undef, 0, fmt.Errorf("failed to convert cid %s from v2 to data commitment: %w", c, err)
		}
		_, pieceCID, err := cid.CidFromBytes(digest)
		if err != nil {
			return cid.Undef, 0, fmt.Errorf("failed to parse %s digest of v2 cid from data commitment: %w", c, err)
		}
		if extractedSize != size {
			return cid.Undef, 0, fmt.Errorf("expected extracted size %d but got %d", size, extractedSize)
		}
		if !pieceCID.Equals(c) {
			return cid.Undef, 0, fmt.Errorf("expected cid %s but got %s", c, pieceCID)
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
			return cid.Undef, 0, fmt.Errorf("filed to compute commp digest: expected %d bytes, got %d", size, written)
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
