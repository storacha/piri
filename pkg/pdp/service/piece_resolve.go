package service

import (
	"context"

	"github.com/multiformats/go-multihash"
)

func (p *PDPService) Resolve(ctx context.Context, data multihash.Multihash) (multihash.Multihash, bool, error) {
	out, found, err := p.pieceResolver.Resolve(ctx, data)
	if err != nil {
		log.Errorw("failed to resolve data", "request", data, "error", err)
	}
	return out, found, err
}

func (p *PDPService) ResolveToBlob(ctx context.Context, piece multihash.Multihash) (multihash.Multihash, bool, error) {
	out, found, err := p.pieceResolver.ResolveToBlob(ctx, piece)
	if err != nil {
		log.Errorw("failed to resolve piece", "request", piece, "error", err)
	}
	return out, found, err
}

func (p *PDPService) ResolveToPiece(ctx context.Context, blob multihash.Multihash) (multihash.Multihash, bool, error) {
	out, found, err := p.pieceResolver.ResolveToPiece(ctx, blob)
	if err != nil {
		log.Errorw("failed to resolve blob", "request", blob, "error", err)
	}
	return out, found, err
}
