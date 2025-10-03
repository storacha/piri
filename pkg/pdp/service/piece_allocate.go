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
	pieceCid, havePieceMapping, err := CommP(allocation.Piece, p.db)
	if err != nil {
		return nil, err
	}

	// Variables to hold information outside the transaction
	var uploadUUID uuid.UUID
	var allocated bool

	if err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if havePieceMapping {
			// Check if a 'parked_pieces' entry exists for the given 'piece_cid'
			// Look up existing parked piece with the given pieceCid, long_term = true, complete = true
			var parkedPiece models.ParkedPiece
			err := tx.Where("piece_cid = ? AND long_term = ? AND complete = ?", pieceCid.String(), true, true).
				First(&parkedPiece).Error

			// If it's neither "record not found" nor nil, it's some other error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("failed to query parked_pieces: %w", err)
			}

			// we already have the piece, bail
			if err == nil {
				// ends transaction
				return nil
			}
		} // else, we got a record not found error looking for the piece, so we don't have it, need to upload

		// Piece does not exist, proceed to create a new upload request
		uploadUUID = uuid.New()

		var pieceCidStr *string
		// if the piece we got back from CommP is defined, the upload was done with either PieceCIDV1 or PieceCIDV2,
		// and we'll know the PieceCID, otherwise we don't have a commp for it and need to calculate at upload
		if !pieceCid.Equals(cid.Undef) {
			ps := pieceCid.String()
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

		allocated = true
		return nil // Commit the transaction

	}); err != nil {
		return nil, err
	}

	if allocated {
		return &types.AllocatedPiece{
			Allocated: true,
			Piece:     pieceCid, // this will either be undefined, or a PieceCIDV2
			UploadID:  uploadUUID,
		}, nil
	}

	return &types.AllocatedPiece{
		Allocated: false,
		Piece:     pieceCid,
		UploadID:  uuid.Nil,
	}, nil
}
