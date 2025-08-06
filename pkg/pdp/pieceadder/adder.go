package pieceadder

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/url"

	"github.com/multiformats/go-multihash"

	"github.com/storacha/piri/pkg/pdp/types"
)

type PieceAdder interface {
	AddPiece(ctx context.Context, digest multihash.Multihash, size uint64) (*url.URL, error)
}

func New(api types.PieceAPI, endpoint *url.URL) *CurioAdder {
	return &CurioAdder{api: api, endpoint: endpoint.JoinPath("pdp", "piece", "upload")}
}

// Generates URLs by interacting with Curio
type CurioAdder struct {
	api      types.PieceAPI
	endpoint *url.URL
}

var _ PieceAdder = (*CurioAdder)(nil)

func (p *CurioAdder) AddPiece(ctx context.Context, digest multihash.Multihash, size uint64) (*url.URL, error) {
	decoded, err := multihash.Decode(digest)
	if err != nil {
		return nil, err
	}
	res, err := p.api.AllocatePiece(ctx, types.PieceAllocation{
		Piece: types.Piece{
			Name: decoded.Name,
			Hash: hex.EncodeToString(decoded.Digest),
			Size: int64(size),
		},
		// TODO use the Notify field to get a notification when the piece is actually added
		//Notify: nil,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add piece: %w", err)
	}
	if !res.Allocated {
		return nil, nil
	}
	return p.endpoint.JoinPath(res.UploadID.String()), nil
}
