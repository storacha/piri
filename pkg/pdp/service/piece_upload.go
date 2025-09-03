package service

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
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
			return types.NewErrorf(types.KindNotFound, "upload ID %s not found", pieceUpload.ID)
		}
		return types.WrapError(types.KindInternal, "failed to query for piece upload", err)
	}

	// PieceRef is a pointer, so a nil value means it's NULL in the DB.
	if upload.PieceRef != nil {
		return types.NewError(types.KindInvalidInput, "piece upload already uploaded")
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
	// NB(forrest): calculation of the commp hash is the bottleneck of the upload process.
	// throughput of the commp hash function on a modern machine is ~1.5Gbps, or less on older
	// machines without a CPU using sha extensions.
	// The uploader supplying the reader that is `pieceUpload.Data` may only send at the rate
	// this host can hash the data.
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

	// Buffer to collect data for PDPStore
	var dataBuffer bytes.Buffer
	limitedReader := io.LimitReader(pieceUpload.Data, maxPieceSize+1) // +1 to detect exceeding the limit
	multiWriter := io.MultiWriter(cp, &dataBuffer)
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
	log.Infow("read piece data", "request", pieceUpload, "size", readSize)

	// Finalize the commP calculation
	digest, paddedPieceSize, err := cp.Digest()
	if err != nil {
		return fmt.Errorf("failed to compute piece hash: %w", err)
	}

	if readSize != upload.CheckSize {
		return fmt.Errorf("piece data does not match the expected size")
	}

	var outHash = digest
	if vhash != nil {
		outHash = vhash.Sum(nil)
	}

	// NB(forrest): here is where we validate the allocated piece actually matches the uploaded piece
	// from this point, writing to storage without verification should be "safe".
	if !bytes.Equal(outHash, upload.CheckHash) {
		return fmt.Errorf("computed hash doe not match expected hash")
	}

	// Convert commP digest into a piece CID
	pieceCIDComputed, err := commcid.DataCommitmentV1ToCID(digest)
	if err != nil {
		return fmt.Errorf("failed to compute piece hash: %w", err)
	}
	log.Infow("computed piece commp", "request", pieceUpload, "commp", pieceCIDComputed.String())

	// Compare the computed piece CID with the expected one from the database
	if upload.PieceCID != nil && pieceCIDComputed.String() != *upload.PieceCID {
		return fmt.Errorf("computer piece CID does not match expected piece CID")
	}

	if err := p.db.Transaction(func(tx *gorm.DB) error {
		// 1. Create a long-term parked piece entry (marked as complete immediately).
		parkedPiece := models.ParkedPiece{
			PieceCID:        pieceCIDComputed.String(),
			PiecePaddedSize: int64(paddedPieceSize),
			PieceRawSize:    readSize,
			LongTerm:        true,
			Complete:        true, // Mark as complete since it's already in PDPStore
		}
		if err := tx.Create(&parkedPiece).Error; err != nil {
			return fmt.Errorf("failed to create %s entry: %w", parkedPiece.TableName(), err)
		}

		// 2. Create a parked piece ref pointing to PDPStore.
		dataURL := fmt.Sprintf("pdpstore://%s", pieceCIDComputed.String())

		parkedPieceRef := models.ParkedPieceRef{
			PieceID:     parkedPiece.ID,
			DataURL:     dataURL,
			LongTerm:    true,
			DataHeaders: datatypes.JSON("{}"), // default empty JSON
		}
		if err := tx.Create(&parkedPieceRef).Error; err != nil {
			return fmt.Errorf("failed to create %s entry: %w", parkedPieceRef.TableName(), err)
		}

		// 3. Optionally insert into pdp_piece_mh_to_commp.
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

		// 4. Move the entry from pdp_piece_uploads to pdp_piecerefs
		ref := models.PDPPieceRef{
			Service:  upload.Service,
			PieceCID: pieceCIDComputed.String(),
			PieceRef: parkedPieceRef.RefID,
		}
		if err := tx.Create(&ref).Error; err != nil {
			return fmt.Errorf("failed to insert into pdp_piecerefs: %w", err)
		}

		// 6. Delete the entry from pdp_piece_uploads
		if err := tx.Delete(&models.PDPPieceUpload{}, "id = ?", upload.ID).Error; err != nil {
			return fmt.Errorf("failed to delete upload ID %s from pdp_piece_uploads: %w", upload.ID, err)
		}

		// Write to PDPStore after successfully creating required database records, if this operation fails, the above tx is rolled back
		if err := p.blobstore.Put(ctx, pieceCIDComputed.Hash(), uint64(readSize), bytes.NewReader(dataBuffer.Bytes())); err != nil {
			return fmt.Errorf("failed to write piece to PDPStore: %w", err)
		}
		log.Infow("wrote piece to PDPStore", "request", pieceUpload, "piece_cid", pieceCIDComputed.String())

		// nil returns will commit the transaction.
		return nil
	}); err != nil {
		return fmt.Errorf("failed to process piece upload: %w", err)
	}

	return nil
}
