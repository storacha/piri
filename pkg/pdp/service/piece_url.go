package service

import (
	"net/url"

	"github.com/google/uuid"
	"github.com/multiformats/go-multihash"
)

func (p *PDPService) ReadPieceURL(piece multihash.Multihash) url.URL {
	return *p.endpoint.JoinPath("piece", piece.HexString())
}

func (p *PDPService) WritePieceURL(id uuid.UUID) url.URL {
	return *p.endpoint.JoinPath("pdp", "piece", "upload", id.String())
}
