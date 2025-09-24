package service

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"

	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/blobstore"
)

func (p *PDPService) ReadPiece(ctx context.Context, piece cid.Cid, options ...types.ReadPieceOption) (res *types.PieceReader, retErr error) {
	cfg := types.ReadPieceConfig{}
	cfg.ProcessOptions(options)

	log.Infow("reading piece", "request", piece)
	defer func() {
		if retErr != nil {
			log.Errorw("failed to read piece", "request", piece, "retErr", retErr)
		} else {
			log.Infow("read piece", "request", piece, "response", res)
		}
	}()

	// TODO(forrest): Nice to have in follow on is attempting to map the `piece` arg to a PieceCIDV2, then
	// performing the query to blobstore with that CID. allowing the read pieces with the cid they allocated them using
	obj, err := p.blobstore.Get(ctx, piece.Hash(), blobstore.WithRange(cfg.ByteRange.Start, cfg.ByteRange.End))

	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, types.NewErrorf(types.KindNotFound, "piece %s not found", piece.String())
		}
		return nil, fmt.Errorf("failed to read piece: %w", err)
	}
	var size int64
	if cfg.ByteRange.Start > 0 || cfg.ByteRange.End != nil {
		start := int64(cfg.ByteRange.Start)
		end := obj.Size() - 1
		if cfg.ByteRange.End != nil {
			end = int64(*cfg.ByteRange.End)
		}
		size = end - start + 1
	} else {
		size = obj.Size()
	}
	return &types.PieceReader{
		Size: size,
		Data: io.NopCloser(obj.Body()),
	}, nil
}
