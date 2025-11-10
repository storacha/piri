package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/google/uuid"
	"github.com/storacha/piri/pkg/store"
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

	// check if we already have this piece
	_, err := p.pieceReader.ReadPiece(ctx, allocation.Piece.Hash)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			// this represents an unexpected error
			return nil, types.WrapError(types.KindInternal, "failed to read piece", err)
		}
		// else it's not found, which is the expected case for new allocations
	} else { // err == nil
		// if we can read the piece, no allocation is required as we already have it.
		return &types.AllocatedPiece{
			Allocated: false,
			Piece:     allocation.Piece.Hash,
			UploadID:  uuid.Nil,
		}, nil
	}

	// Variables to hold information outside the transaction
	var uploadUUID uuid.UUID

	if err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Piece does not exist, proceed to create a new upload request
		uploadUUID = uuid.New()

		notifyURL := ""
		if allocation.Notify != nil {
			notifyURL = allocation.Notify.String()
		}

		newUpload := &models.PDPPieceUpload{
			ID:             uploadUUID.String(),
			Service:        "storacha",
			NotifyURL:      notifyURL,
			CheckHashCodec: allocation.Piece.Name,
			CheckHash:      allocation.Piece.Hash,
			CheckSize:      allocation.Piece.Size,
		}
		if createErr := tx.Create(&newUpload).Error; createErr != nil {
			return fmt.Errorf("failed to store upload request in database: %w", createErr)
		}

		return nil // Commit the transaction

	}); err != nil {
		return nil, err
	}

	return &types.AllocatedPiece{
		Allocated: true,
		Piece:     allocation.Piece.Hash,
		UploadID:  uploadUUID,
	}, nil

}
