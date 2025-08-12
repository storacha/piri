package service

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"

	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store"
)

func (p *PDPService) ReadPiece(ctx context.Context, piece cid.Cid) (res *types.PieceReader, retErr error) {
	log.Infow("reading piece", "request", piece)
	defer func() {
		if retErr != nil {
			log.Errorw("failed to read piece", "request", piece, "retErr", retErr)
		} else {
			log.Infow("read piece", "request", piece, "response", res)
		}
	}()
	obj, err := p.blobstore.Get(ctx, piece.Hash())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, types.NewErrorf(types.KindNotFound, "piece %s not found", piece.String())
		}
		return nil, fmt.Errorf("failed to read piece: %w", err)
	}
	return &types.PieceReader{
		Size: obj.Size(),
		Data: io.NopCloser(obj.Body()),
	}, nil
}
