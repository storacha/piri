package service

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/types"
)

func (p *PDPService) FindPiece(ctx context.Context, piece types.Piece) (_ cid.Cid, _ bool, retErr error) {
	log.Infow("finding piece", "request", piece)
	defer func() {
		if retErr != nil {
			log.Errorw("failed to find piece", "request", piece, "error", retErr)
		}
	}()
	pieceCID, havePieceMapping, err := CommP(piece, p.db)
	if err != nil {
		return cid.Undef, false, err
	}

	// we don't have a mapping from the uploaded hash function to the PieceCID,
	//and the PieceCID we got back isn't defined - we 100% don't have this piece
	if !havePieceMapping && pieceCID.Equals(cid.Undef) {
		return cid.Undef, false, nil
	}
	// otherwise, we have a mapping, or the piece was a PieceCIDV2, check if uploaded

	var count int64
	if err := p.db.WithContext(ctx).Model(&models.ParkedPiece{}).
		Where("piece_cid = ? AND long_term = ? AND complete = ?", pieceCID.String(), true, true).
		Count(&count).Error; err != nil {
		return cid.Undef, false, fmt.Errorf("failed to find count parked pieces: %w", err)
	}
	if count == 0 {
		// no error needed, simply not found
		return cid.Undef, false, nil
	}

	return pieceCID, true, nil
}
