package service

import (
	"context"

	"github.com/multiformats/go-multihash"
)

func (p *PDPService) ResolvePiece(ctx context.Context, piece multihash.Multihash) (_ multihash.Multihash, _ bool, retErr error) {
	log.Debugw("resolving piece", "request", piece)
	defer func() {
		if retErr != nil {
			log.Errorw("failed to resolve piece", "request", piece, "error", retErr)
		}
	}()

	return p.pieceResolver.ResolvePiece(ctx, piece)
}
