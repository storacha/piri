package service

import (
	"context"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"

	"github.com/storacha/piri/pkg/pdp/types"
)

func (p *PDPService) ReadPiece(ctx context.Context, piece cid.Cid) (*types.PieceReader, error) {
	obj, err := p.blobstore.Get(ctx, piece.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to read piece: %w", err)
	}
	return &types.PieceReader{
		Size: obj.Size(),
		Data: io.NopCloser(obj.Body()),
	}, nil
}
