package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/store"
	"gorm.io/gorm"
)

// resolvePieceInternal is the shared implementation for both FindPiece and HasPiece
func (p *PDPService) resolvePieceInternal(ctx context.Context, piece cid.Cid) (cid.Cid, bool, error) {
	// Check what type of piece this is
	switch piece.Prefix().MhType {
	case uint64(multicodec.Fr32Sha256Trunc254Padbintree): // PieceCIDV2
		var record models.PDPPieceMHToCommp
		if err := p.db.Where("commp = ?", piece.String()).First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// no mapping, could have been uploaded as commp
				if _, err := p.blobstore.Get(ctx, piece.Hash()); err != nil {
					if errors.Is(err, store.ErrNotFound) {
						return cid.Undef, false, nil
					} else {
						return cid.Undef, false, fmt.Errorf("failed to read blobstore: %w", err)
					}
				}
			} else {
				return cid.Undef, false, fmt.Errorf("failed to read database: %w", err)
			}
		}
		// found a record!
		read, mh, err := multihash.MHFromBytes(record.Mhash)
		if err != nil {
			return cid.Undef, false, fmt.Errorf("failed to read multihash: %w", err)
		}
		if read != len(record.Mhash) {
			return cid.Undef, false, fmt.Errorf("multihash read mismatch expected %d got %d", len(record.Mhash), read)
		}
		// TODO now we could be extra paranoid and check the blobstore too, but the expectation is these are in sync
		dmh, err := multihash.Decode(mh)
		if err != nil {
			return cid.Undef, false, fmt.Errorf("failed to decode multihash: %w", err)
		}
		return cid.NewCidV1(dmh.Code, dmh.Digest), true, nil
	default:
		// This is not a commp piece, it's a regular piece with a different hash function

		// Check if it exists in the blobstore
		if _, err := p.blobstore.Get(ctx, piece.Hash()); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return cid.Undef, false, nil
			}
			return cid.Undef, false, fmt.Errorf("failed to check blobstore: %w", err)
		}

		// The piece exists in blobstore, return the CID that can be used to read it
		return piece, true, nil
	}
}

func (p *PDPService) ResolvePiece(ctx context.Context, piece cid.Cid) (_ cid.Cid, _ bool, retErr error) {
	log.Debugw("resolving piece", "request", piece)
	defer func() {
		if retErr != nil {
			log.Errorw("failed to resolve piece", "request", piece, "error", retErr)
		}
	}()

	return p.resolvePieceInternal(ctx, piece)
}
