package service

import (
	"context"

	"github.com/ipfs/go-cid"
)

func (p *PDPService) HasPiece(ctx context.Context, piece cid.Cid) (bool, error) {
	_, exists, err := p.resolvePieceInternal(ctx, piece)
	return exists, err
}
