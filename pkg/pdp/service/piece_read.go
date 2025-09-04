package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/ipfs/go-cid"

	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store"
)

// trackingReadCloser wraps a ReadCloser to track active downloads
type trackingReadCloser struct {
	io.ReadCloser
	counter  *atomic.Int32
	pieceCid cid.Cid
	closed   atomic.Bool
}

func (t *trackingReadCloser) Close() error {
	// Ensure we only decrement once
	if t.closed.CompareAndSwap(false, true) {
		count := t.counter.Add(-1)
		log.Infow("download completed", "piece", t.pieceCid, "active_downloads", count)
	}
	return t.ReadCloser.Close()
}

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
	
	// Increment active downloads counter and wrap the reader
	count := p.activeDownloads.Add(1)
	log.Infow("download started", "piece", piece, "active_downloads", count)
	
	wrappedReader := &trackingReadCloser{
		ReadCloser: io.NopCloser(obj.Body()),
		counter:    p.activeDownloads,
		pieceCid:   piece,
	}
	
	return &types.PieceReader{
		Size: obj.Size(),
		Data: wrappedReader,
	}, nil
}
