package service

import (
	"net/url"

	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
)

func (p *PDPService) ReadPieceURL(piece cid.Cid) (url.URL, error) {
	return *p.endpoint.JoinPath("piece", piece.String()), nil
}

func (p *PDPService) WritePieceURL(id uuid.UUID) (url.URL, error) {
	return *p.endpoint.JoinPath("pdp", "piece", "upload", id.String()), nil
}
