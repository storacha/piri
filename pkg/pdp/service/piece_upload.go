package service

import (
	"context"
	"errors"
	"fmt"
	"hash"

	"github.com/storacha/piri/lib/verifyread"
	"gorm.io/gorm"

	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/multiformats/go-multihash"
	mhreg "github.com/multiformats/go-multihash/core"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/types"
)

func (p *PDPService) UploadPiece(ctx context.Context, pieceUpload types.PieceUpload) (retErr error) {
	var upload models.PDPPieceUpload
	if err := p.db.First(&upload, "id = ?", pieceUpload.ID.String()).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return types.NewErrorf(types.KindNotFound, "upload ID %s not found", pieceUpload.ID)
		}
		return types.WrapError(types.KindInternal, "failed to query for piece upload", err)
	}

	var hasher hash.Hash
	if upload.CheckHashCodec == multihash.Codes[multihash.SHA2_256_TRUNC254_PADDED] {
		hasher = &commp.Calc{}
	} else {
		// TODO(forrest): I bet the commp hash function isn't returned by this, so above special case
		var err error
		hasher, err = mhreg.GetVariableHasher(multihash.Names[upload.CheckHashCodec], -1)
		if err != nil {
			return types.WrapError(types.KindInvalidInput, fmt.Sprintf("unknown hash coded: %s", upload.CheckHashCodec), err)
		}
	}

	if err := p.db.Transaction(func(tx *gorm.DB) error {
		// transaction since we only want to remove the upload entry if we can write to the store
		if err := tx.Delete(&models.PDPPieceUpload{}, "id = ?", upload.ID).Error; err != nil {
			return types.WrapError(types.KindInternal, fmt.Sprintf("failed to delete piece upload ID %s from pdp_piece_uploads", upload.ID), err)
		}

		mh, err := multihash.Decode(upload.CheckHash)
		if err != nil {
			return types.WrapError(types.KindInternal, "failed to decode check hash", err)
		}

		if err := p.blobstore.Put(ctx, upload.CheckHash, uint64(upload.CheckSize), verifyread.New(pieceUpload.Data, hasher, mh.Digest)); err != nil {
			return types.WrapError(types.KindInvalidInput, "failed to put piece", err)
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}
