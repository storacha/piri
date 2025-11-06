package service

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/google/uuid"
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
	mh, err := hex.DecodeString(allocation.Piece.Hash)
	if err != nil {
		return nil, types.WrapError(types.KindInvalidInput, "failed to decode piece hash", err)
	}
	_, has, err := p.pieceResolver.ResolvePiece(ctx, mh)
	if err != nil {
		return nil, types.WrapError(types.KindInternal, "failed to resolve piece", err)
	}

	// if we have it no allocation needed
	if has {
		return &types.AllocatedPiece{
			Allocated: false,
			Piece:     mh,
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
			PieceCID:       nil, // TODO fields never used in practice, need a migration away from this
			NotifyURL:      notifyURL,
			CheckHashCodec: allocation.Piece.Name,
			CheckHash:      must.One(hex.DecodeString(allocation.Piece.Hash)),
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
		Piece:     mh,
		UploadID:  uploadUUID,
	}, nil

}
