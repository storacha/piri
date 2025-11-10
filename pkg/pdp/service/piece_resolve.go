package service

import (
	"context"

	"github.com/multiformats/go-multihash"
)

func (p *PDPService) ResolvePiece(ctx context.Context, piece multihash.Multihash) (multihash.Multihash, bool, error) {
	out, found, err := p.pieceResolver.ResolvePiece(ctx, piece)
	if err != nil {
		log.Errorw("failed to resolve piece", "request", piece, "error", err)
	}
	return out, found, err
}
