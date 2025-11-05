package service

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/multiformats/go-multihash"

	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/blobstore"
)

func (p *PDPService) ReadPiece(ctx context.Context, piece multihash.Multihash, options ...types.ReadPieceOption) (res *types.PieceReader, retErr error) {
	// TODO may want to make this an option...
	readCID, found, err := p.resolvePieceInternal(ctx, piece)
	if err != nil {
		return nil, fmt.Errorf("failed to find piece: %w", err)
	}
	if !found {
		return nil, types.NewErrorf(types.KindNotFound, "piece %s not found", piece.String())
	}

	cfg := types.ReadPieceConfig{}
	cfg.ProcessOptions(options)

	log.Debugw("reading piece", "request", piece, "resolved", readCID)
	defer func() {
		if retErr != nil {
			log.Errorw("failed to read piece", "request", piece, "resolved", readCID, "retErr", retErr)
		} else {
			log.Debugw("read piece", "request", piece, "resolved", readCID, "response", res)
		}
	}()

	var getOptions []blobstore.GetOption
	if cfg.ByteRange.Start > 0 || cfg.ByteRange.End != nil {
		getOptions = append(getOptions, blobstore.WithRange(cfg.ByteRange.Start, cfg.ByteRange.End))
	}

	obj, err := p.blobstore.Get(ctx, readCID, getOptions...)

	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, types.NewErrorf(types.KindNotFound, "piece %s not found", readCID.String())
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
