package service

import (
	"context"
	"errors"

	"github.com/multiformats/go-multihash"

	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store"
)

func (p *PDPService) Read(ctx context.Context, data multihash.Multihash, options ...types.ReadPieceOption) (res *types.PieceReader, retErr error) {
	log.Debugw("reading data", "request", data)
	defer func() {
		if retErr != nil {
			log.Errorw("failed to read data", "request", data, "retErr", retErr)
		} else {
			log.Debugw("read data", "request", data, "response", res)
		}
	}()

	pr, err := p.pieceReader.Read(ctx, data, options...)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, types.NewError(types.KindNotFound, "data not found")
		}
		return nil, types.WrapError(types.KindInternal, "failed to read data", err)
	}
	return pr, nil
}

func (p *PDPService) Has(ctx context.Context, blob multihash.Multihash) (bool, error) {
	has, err := p.pieceReader.Has(ctx, blob)
	if err != nil {
		return false, types.WrapError(types.KindInternal, "failed to read data", err)
	}
	return has, nil
}
