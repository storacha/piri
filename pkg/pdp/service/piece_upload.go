package service

import (
	"context"
	"errors"
	"fmt"

	commcid "github.com/filecoin-project/go-fil-commcid"
	"github.com/hashicorp/go-multierror"
	"github.com/multiformats/go-multicodec"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/storacha/piri/lib/verifyread"
	"github.com/storacha/piri/pkg/pdp/piece"
	"github.com/storacha/piri/pkg/presets"

	"github.com/multiformats/go-multihash"

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
	lg := log.With("upload_id", pieceUpload.ID, "digest", multihash.Multihash(upload.CheckHash).String(), "size", upload.CheckSize)

	hasher, ok := presets.HasherRegistry[upload.CheckHashCodec]
	if !ok {
		return types.NewErrorf(types.KindInvalidInput, "unknown hash code: %s", upload.CheckHashCodec)
	}

	mh, err := multihash.Decode(upload.CheckHash)
	if err != nil {
		return types.WrapError(types.KindInternal, "failed to decode check hash", err)
	}

	vr, err := verifyread.New(pieceUpload.Data, hasher(), mh.Digest)
	if err != nil {
		return types.WrapError(types.KindInternal, "failed to create verification reader", err)
	}

	if err := p.blobstore.Put(ctx, upload.CheckHash, uint64(upload.CheckSize), vr); err != nil {
		lg.Errorw("failed to write upload to blobstore", "err", err)
		return types.WrapError(types.KindInvalidInput, "failed to put piece", err)
	}

	if err := p.db.Transaction(func(tx *gorm.DB) error {
		// transaction since we only want to remove the upload entry if we can write to the store
		if err := tx.Delete(&models.PDPPieceUpload{}, "id = ?", upload.ID).Error; err != nil {
			return types.WrapError(types.KindInternal, fmt.Sprintf("failed to delete piece upload ID %s from pdp_piece_uploads", upload.ID), err)
		}

		// if the upload was done with commp create a mapping for it now
		if upload.CheckHashCodec == multicodec.Fr32Sha256Trunc254Padbintree.String() {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
				Create(&models.PDPPieceMHToCommp{
					Mhash: upload.CheckHash,
					Size:  upload.CheckSize,
					Commp: piece.MultihashToCommpCID(upload.CheckHash).String(),
				}).Error; err != nil {
				return types.WrapError(types.KindInternal, "failed to create pieceMH to commp", err)
			}
		} else if upload.CheckHashCodec == multicodec.Sha2_256Trunc254Padded.String() {
			pv1, err := commcid.DataCommitmentV1ToCID(mh.Digest)
			if err != nil {
				return err
			}
			pieceCID, err := commcid.PieceCidV2FromV1(pv1, uint64(upload.CheckSize))
			if err != nil {
				return fmt.Errorf("failed to convert pieceCid %s from v1 to v2: %w", pv1, err)
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
				Create(&models.PDPPieceMHToCommp{
					Mhash: upload.CheckHash,
					Size:  upload.CheckSize,
					Commp: piece.MultihashToCommpCID(pieceCID.Hash()).String(),
				}).Error; err != nil {
				return types.WrapError(types.KindInternal, "failed to create pieceMH to commp", err)
			}
		}

		return nil
	}); err != nil {
		merr := new(multierror.Error)
		merr = multierror.Append(merr, err)

		lg.Errorw("failed to persist database records for piece upload", "err", err)
		// we write the data to the blobstore before the transaction that records its metadata in the task engineDB
		// if the transaction fails for whatever reason then we need to delete it from the blobstore
		if delErr := p.blobstore.Delete(ctx, upload.CheckHash); delErr != nil {
			lg.Errorw("failed to delete data from blobstore for failed upload", "err", delErr)
			merr = multierror.Append(merr, delErr)
		}
		return merr.ErrorOrNil()
	}

	return nil
}
