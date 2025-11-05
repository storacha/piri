package service

import (
	"context"

	"github.com/ipfs/go-cid"
)

// resolvePieceInternal is the shared implementation for both FindPiece and HasPiece
func (p *PDPService) resolvePieceInternal(ctx context.Context, piece cid.Cid) (cid.Cid, bool, error) {
	return p.pieceResolver.Resolve(ctx, piece)
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
