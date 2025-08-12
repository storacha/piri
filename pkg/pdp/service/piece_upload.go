package service

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	commcid "github.com/filecoin-project/go-fil-commcid"
	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/multiformats/go-multihash"
	mhreg "github.com/multiformats/go-multihash/core"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/types"
)

func (p *PDPService) UploadPiece(ctx context.Context, pieceUpload types.PieceUpload) (retErr error) {
	start := time.Now()
	log.Infow("uploading piece", "request", pieceUpload)
	defer func() {
		if retErr != nil {
			log.Errorw("failed to upload piece", "request", pieceUpload, "duration", time.Since(start), "error", retErr)
		} else {
			log.Infow("uploaded piece", "request", pieceUpload, "duration", time.Since(start))
		}
	}()
	// Lookup the expected pieceCID, notify_url, and piece_ref from the database using uploadUUID
	var upload models.PDPPieceUpload
	if err := p.db.First(&upload, "id = ?", pieceUpload.ID.String()).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("upload UUID not found")
		}
		return fmt.Errorf("database error: %w", err)
	}

	// PieceRef is a pointer, so a nil value means it's NULL in the DB.
	if upload.PieceRef != nil {
		return fmt.Errorf("data has already been uploaded")
	}

	ph := types.Piece{
		Name: upload.CheckHashCodec,
		Hash: hex.EncodeToString(upload.CheckHash),
		Size: upload.CheckSize,
	}
	phMh, err := Multihash(ph)
	if err != nil {
		return err
	}

	// Limit the size of the piece data
	maxPieceSize := upload.CheckSize

	// Create a commp.Calc instance for calculating commP
	cp := &commp.Calc{}
	readSize := int64(0)

	var vhash hash.Hash
	if upload.CheckHashCodec != multihash.Codes[multihash.SHA2_256_TRUNC254_PADDED] {
		hasher, err := mhreg.GetVariableHasher(multihash.Names[upload.CheckHashCodec], -1)
		if err != nil {
			return fmt.Errorf("failed to get hasher: %w", err)
		}
		vhash = hasher
	}

	// Function to write data into StashStore and calculate commP
	writeFunc := func(f *os.File) error {
		limitedReader := io.LimitReader(pieceUpload.Data, maxPieceSize+1) // +1 to detect exceeding the limit
		multiWriter := io.MultiWriter(cp, f)
		if vhash != nil {
			multiWriter = io.MultiWriter(vhash, multiWriter)
		}

		// Copy data from limitedReader to multiWriter
		n, err := io.Copy(multiWriter, limitedReader)
		if err != nil {
			return fmt.Errorf("failed to read and write piece data: %w", err)
		}

		if n > maxPieceSize {
			return fmt.Errorf("piece data exceeds the maximum allowed size")
		}

		readSize = n

		log.Infow("wrote piece", "request", pieceUpload, "duration", time.Since(start))
		return nil
	}

	// Upload into StashStore
	stashID, err := p.storage.StashCreate(ctx, maxPieceSize, writeFunc)
	if err != nil {
		return fmt.Errorf("failed to create stash: %w", err)
	}

	// Finalize the commP calculation
	digest, paddedPieceSize, err := cp.Digest()
	if err != nil {
		// Remove the stash file as the data is invalid
		_ = p.storage.StashRemove(ctx, stashID)
		return fmt.Errorf("failed to compute piece hash: %w", err)
	}

	if readSize != upload.CheckSize {
		_ = p.storage.StashRemove(ctx, stashID)
		return fmt.Errorf("piece data does not match the expected size")
	}

	var outHash = digest
	if vhash != nil {
		outHash = vhash.Sum(nil)
	}

	if !bytes.Equal(outHash, upload.CheckHash) {
		// Remove the stash file as the data is invalid
		_ = p.storage.StashRemove(ctx, stashID)
		return fmt.Errorf("computed hash doe not match expected hash")
	}

	// Convert commP digest into a piece CID
	pieceCIDComputed, err := commcid.DataCommitmentV1ToCID(digest)
	if err != nil {
		// Remove the stash file as the data is invalid
		_ = p.storage.StashRemove(ctx, stashID)
		return fmt.Errorf("failed to compute piece hash: %w", err)
	}
	log.Infow("computed piece commp", "request", pieceUpload, "commp", pieceCIDComputed.String(), "duration", time.Since(start))

	// Compare the computed piece CID with the expected one from the database
	if upload.PieceCID != nil && pieceCIDComputed.String() != *upload.PieceCID {
		// Remove the stash file as the data is invalid
		_ = p.storage.StashRemove(ctx, stashID)
		return fmt.Errorf("computer piece CID does not match expected piece CID")
	}

	if err := p.db.Transaction(func(tx *gorm.DB) error {
		// 1. Create a long-term parked piece entry.
		parkedPiece := models.ParkedPiece{
			PieceCID:        pieceCIDComputed.String(),
			PiecePaddedSize: int64(paddedPieceSize),
			PieceRawSize:    readSize,
			LongTerm:        true,
		}
		if err := tx.Create(&parkedPiece).Error; err != nil {
			return fmt.Errorf("failed to create %s entry: %w", parkedPiece.TableName(), err)
		}

		// 2. Create a parked piece ref.
		stashURL, err := p.storage.StashURL(stashID)
		if err != nil {
			return fmt.Errorf("failed to get stash URL: %w", err)
		}
		dataURL := stashURL.String()

		parkedPieceRef := models.ParkedPieceRef{
			PieceID:     parkedPiece.ID,
			DataURL:     dataURL,
			LongTerm:    true,
			DataHeaders: datatypes.JSON("{}"), // default empty JSON
		}
		if err := tx.Create(&parkedPieceRef).Error; err != nil {
			return fmt.Errorf("failed to create %s entry: %w", parkedPieceRef.TableName(), err)
		}

		// 3. Update the pdp_piece_uploads entry.
		if err := tx.Model(&models.PDPPieceUpload{}).
			Where("id = ?", pieceUpload.ID.String()).
			Updates(map[string]interface{}{
				"piece_ref": parkedPieceRef.RefID,
				"piece_cid": pieceCIDComputed.String(),
			}).Error; err != nil {
			return fmt.Errorf("failed to update %s: %w", models.PDPPieceUpload{}.TableName(), err)
		}

		// 4. Optionally insert into pdp_piece_mh_to_commp.
		if upload.CheckHashCodec != multihash.Codes[multihash.SHA2_256_TRUNC254_PADDED] {
			// Define a local model for the table.
			mhToCommp := models.PDPPieceMHToCommp{
				Mhash: phMh,
				Size:  upload.CheckSize,
				Commp: pieceCIDComputed.String(),
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&mhToCommp).Error; err != nil {
				return fmt.Errorf("failed to insert into %s: %w", mhToCommp.TableName(), err)
			}
		}

		// nil returns will commit the transaction.
		return nil
	}); err != nil {
		// Remove the stash file as the transaction failed
		_ = p.storage.StashRemove(ctx, stashID)
		return fmt.Errorf("failed to process piece upload: %w", err)

	}

	return nil
}
