package service

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/types"
)

func (p *PDPService) FindPiece(ctx context.Context, piece types.Piece) (cid.Cid, bool, error) {
	pieceCID, havePieceCid, err := CommP(piece, p.db)
	if err != nil {
		return cid.Undef, false, err
	}

	// upload either not complete or does not exist
	if !havePieceCid {
		return cid.Undef, false, nil
	}
	// Verify that a 'parked_pieces' entry exists for the given 'piece_cid'
	// NB: the storacha node currently polls this method until it gets a positive conformation
	// the piece exists in the parked_piece table, we could alternativly remove the async nature of this task
	// when we are happy with the overall port of curio.
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
