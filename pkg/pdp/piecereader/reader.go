package piecereader

import (
	"context"
	"net/url"

	"github.com/ipfs/go-cid"

	"github.com/storacha/piri/pkg/pdp/types"
)

type PieceReader interface {
	ReadPiece(ctx context.Context, piece cid.Cid, options ...types.ReadPieceOption) (*types.PieceReader, error)
}

var _ PieceReader = (*CurioReader)(nil)

type CurioReader struct {
	api      types.PieceAPI
	endpoint *url.URL
}

func New(api types.PieceAPI, endpoint *url.URL) *CurioReader {
	return &CurioReader{api, endpoint.JoinPath("piece")}
}

func (r *CurioReader) ReadPiece(ctx context.Context, piece cid.Cid, options ...types.ReadPieceOption) (*types.PieceReader, error) {
	return r.api.ReadPiece(ctx, piece, options...)
}
