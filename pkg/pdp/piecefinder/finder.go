package piecefinder

import (
	"context"
	"net/url"
	"time"

	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/piece/piece"

	"github.com/storacha/piri/pkg/pdp/types"
)

type PieceFinder interface {
	FindPiece(ctx context.Context, digest multihash.Multihash, size uint64) (piece.PieceLink, error)
	HasPiece(ctx context.Context, digest multihash.Multihash, size uint64) (bool, error)
	URLForPiece(ctx context.Context, piece multihash.Multihash) (url.URL, error)
}

var _ PieceFinder = (*CurioFinder)(nil)

type CurioFinder struct {
	api      types.PieceAPI
	endpoint *url.URL

	maxAttempts int
	retryDelay  time.Duration
}

type Option func(cf *CurioFinder)

func WithRetryDelay(d time.Duration) Option {
	return func(cf *CurioFinder) {
		cf.retryDelay = d
	}
}

func WithMaxAttempts(n int) Option {
	return func(cf *CurioFinder) {
		cf.maxAttempts = n
	}
}

const defaultMaxAttempts = 10
const defaultRetryDelay = 5 * time.Second

func New(api types.PieceAPI, endpoint *url.URL, opts ...Option) *CurioFinder {
	cf := &CurioFinder{
		api:         api,
		endpoint:    endpoint.JoinPath("piece"),
		maxAttempts: defaultMaxAttempts,
		retryDelay:  defaultRetryDelay,
	}

	for _, opt := range opts {
		opt(cf)
	}
	return cf
}

// GetDownloadURL implements access.Access.
func (a *CurioFinder) FindPiece(ctx context.Context, digest multihash.Multihash, size uint64) (piece.PieceLink, error) {
	panic("TODO)")
	/*
		// TODO: improve this. @magik6k says curio will have piece ready for processing
		// in seconds, but we're not sure how long that will be. We need to iterate on this
		// till we have a better solution
		attempts := 0
		for {
			pieceCID, found, err := a.api.ResolvePiece(ctx, digest)
			// NB: an error here indicates a critical failure, if the piece isn't found, no error is returned.
			if err != nil {
				return nil, fmt.Errorf("finding piece: %w", err)
			}
			if found {
				return piece.FromLink(cidlink.Link{Cid: pieceCID})
			}
			// piece not found, try again
			attempts++
			if attempts >= a.maxAttempts {
				return nil, fmt.Errorf("maximum retries exceeded: %w", store.ErrNotFound)
			}
			timer := time.NewTimer(a.retryDelay)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-timer.C:
			}
		}

	*/
}

func (a *CurioFinder) URLForPiece(ctx context.Context, p multihash.Multihash) (url.URL, error) {
	return *a.endpoint.JoinPath(p.B58String()), nil
}

func (a *CurioFinder) HasPiece(ctx context.Context, digest multihash.Multihash, size uint64) (bool, error) {
	return a.api.HasPiece(ctx, digest)
}
