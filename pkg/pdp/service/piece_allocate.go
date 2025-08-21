package service

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/snadrus/must"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/types"
)

func (p *PDPService) AllocatePiece(ctx context.Context, allocation types.PieceAllocation) (res *types.AllocatedPiece, retErr error) {
	log.Infow("allocating piece", "request", allocation)
	defer func() {
		if retErr != nil {
			log.Errorw("failed to allocate piece", "request", allocation, "err", retErr)
		} else {
			log.Infow("allocated piece", "request", allocation, "response", res)
		}
	}()
	if abi.UnpaddedPieceSize(allocation.Piece.Size) > PieceSizeLimit {
		return nil, types.NewErrorf(types.KindInvalidInput, "piece size %d exceeds limit %d", allocation.Piece.Size, PieceSizeLimit)
	}

	// map pieceCID, if sha256, to filecoin commp cid
	pieceCid, havePieceCid, err := CommP(allocation.Piece, p.db)
	if err != nil {
		return nil, err
	}

	// Variables to hold information outside the transaction
	var uploadUUID uuid.UUID
	var created bool

	if err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if havePieceCid {
			// Check if a 'parked_pieces' entry exists for the given 'piece_cid'
			// Look up existing parked piece with the given pieceCid, long_term = true, complete = true
			var parkedPiece models.ParkedPiece
			err := tx.Where("piece_cid = ? AND long_term = ? AND complete = ?", pieceCid.String(), true, true).
				First(&parkedPiece).Error

			// If it's neither "record not found" nor nil, it's some other error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("failed to query parked_pieces: %w", err)
			}

			if err == nil {
				// Create a new parked_piece_refs entry referencing the existing piece
				parkedRef := &models.ParkedPieceRef{
					PieceID:  parkedPiece.ID,
					LongTerm: true,
				}
				if createErr := tx.Create(&parkedRef).Error; createErr != nil {
					return fmt.Errorf("failed to insert into parked_piece_refs: %w", createErr)
				}

				// Create the pdp_piece_uploads record pointing to the parked_piece_refs entry
				uploadUUID = uuid.New()
				upload := &models.PDPPieceUpload{
					ID:       uploadUUID.String(),
					Service:  "storacha",
					PieceCID: models.Ptr(pieceCid.String()),
					NotifyURL: func() string {
						if allocation.Notify == nil {
							return ""
						}
						return allocation.Notify.String()
					}(),
					PieceRef:       &parkedRef.RefID,
					CheckHashCodec: allocation.Piece.Name,
					CheckHash:      must.One(hex.DecodeString(allocation.Piece.Hash)),
					CheckSize:      allocation.Piece.Size,
				}
				if createErr := tx.Create(&upload).Error; createErr != nil {
					return fmt.Errorf("failed to insert into pdp_piece_uploads: %w", createErr)
				}

				// ends transaction
				return nil
			}
		} // else

		// Piece does not exist, proceed to create a new upload request
		uploadUUID = uuid.New()

		// Store the upload request in the database
		var pieceCidStr *string
		if p, ok := MaybeStaticCommp(allocation.Piece); ok {
			ps := p.String()
			pieceCidStr = &ps
		}
		notifyURL := ""
		if allocation.Notify != nil {
			notifyURL = allocation.Notify.String()
		}

		newUpload := &models.PDPPieceUpload{
			ID:             uploadUUID.String(),
			Service:        "storacha",
			PieceCID:       pieceCidStr, // might be empty if no static commP
			NotifyURL:      notifyURL,
			CheckHashCodec: allocation.Piece.Name,
			CheckHash:      must.One(hex.DecodeString(allocation.Piece.Hash)),
			CheckSize:      allocation.Piece.Size,
		}
		if createErr := tx.Create(&newUpload).Error; createErr != nil {
			return fmt.Errorf("failed to store upload request in database: %w", createErr)
		}

		created = true
		return nil // Commit the transaction

	}); err != nil {
		return nil, err
	}

	if created {
		return &types.AllocatedPiece{
			Allocated: true,
			Piece:     cid.Undef,
			UploadID:  uploadUUID,
		}, nil
	}

	return &types.AllocatedPiece{
		Allocated: false,
		Piece:     pieceCid,
		UploadID:  uuid.Nil,
	}, nil
}
