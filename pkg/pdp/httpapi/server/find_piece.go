package server

import (
	"encoding/hex"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/storacha/piri/pkg/pdp/httpapi"
)

func (p *PDPHandler) handleFindPiece(c echo.Context) error {
	ctx := c.Request().Context()

	name := c.QueryParam("name")
	if name == "" {
		return c.String(http.StatusBadRequest, "name is required")
	}
	hash := c.QueryParam("hash")
	if hash == "" {
		return c.String(http.StatusBadRequest, "hash is required")
	}
	mh, err := hex.DecodeString(hash)
	if err != nil {
		return c.String(http.StatusBadRequest, "hash is invalid")
	}

	// Verify that a 'parked_pieces' entry exists for the given 'piece_cid'
	pieceCID, has, err := p.Service.Resolve(ctx, mh)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to find piece in database")
	}
	if !has {
		return c.String(http.StatusNotFound, "piece not found")
	}

	resp := httpapi.FoundPieceResponse{
		PieceCID: pieceCID.String(),
	}

	return c.JSON(http.StatusOK, resp)
}
