package service

import (
	"context"

	"github.com/multiformats/go-multihash"
)

func (p *PDPService) HasPiece(ctx context.Context, piece multihash.Multihash) (bool, error) {
	_, exists, err := p.resolvePieceInternal(ctx, piece)
	return exists, err
}
