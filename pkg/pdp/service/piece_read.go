package service

import (
	"context"
	"errors"

	"github.com/multiformats/go-multihash"

	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store"
)

func (p *PDPService) ReadPiece(ctx context.Context, piece multihash.Multihash, options ...types.ReadPieceOption) (res *types.PieceReader, retErr error) {
	log.Debugw("reading piece", "request", piece)
	defer func() {
		if retErr != nil {
			log.Errorw("failed to read piece", "request", piece, "retErr", retErr)
		} else {
			log.Debugw("read piece", "request", piece, "response", res)
		}
	}()

	pr, err := p.pieceReader.ReadPiece(ctx, piece)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, types.NewError(types.KindNotFound, "piece not found")
		}
		return nil, types.WrapError(types.KindInternal, "failed to read piece", err)
	}
	return pr, nil
}
