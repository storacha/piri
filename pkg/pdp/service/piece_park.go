package service

import (
	"context"
	"fmt"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/types"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (p *PDPService) ParkPiece(ctx context.Context, params types.ParkPieceRequest) error {
	if err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Create a long-term parked piece entry (marked as complete immediately).
		parkedPiece := models.ParkedPiece{
			PieceCID:        params.PieceCID.String(),
			PiecePaddedSize: int64(params.PaddedSize),
			PieceRawSize:    int64(params.RawSize),
			LongTerm:        true,
			Complete:        true, // Mark as complete since it's already in PDPStore
		}
		if err := tx.Create(&parkedPiece).Error; err != nil {
			return fmt.Errorf("failed to create %s entry: %w", parkedPiece.TableName(), err)
		}

		// 2. Create a parked piece ref pointing to PDPStore.
		// NB this field is meaningless, but we might want to use the multihash for the value
		// since that's the key in the store
		dataURL := fmt.Sprintf("pdpstore://%s", params.Blob.String())

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
		if params.PieceCID.Hash().HexString() != params.Blob.HexString() {
			mhToCommp := models.PDPPieceMHToCommp{
				Mhash: params.Blob,
				Size:  int64(params.RawSize),
				Commp: params.PieceCID.String(),
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&mhToCommp).Error; err != nil {
				return fmt.Errorf("failed to insert into %s: %w", mhToCommp.TableName(), err)
			}
		}

		// 4. Create a reference in pdp_piecerefs
		ref := models.PDPPieceRef{
			Service:  "storacha",
			PieceCID: params.PieceCID.String(),
			PieceRef: parkedPieceRef.RefID,
		}
		if err := tx.Create(&ref).Error; err != nil {
			return fmt.Errorf("failed to create %s entry: %w", ref.TableName(), err)
		}

		// nil returns will commit the transaction.
		return nil
	}); err != nil {
		return fmt.Errorf("failed to park piece: %w", err)
	}

	return nil
}
