package server

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/pdp/httpapi"
	"github.com/storacha/piri/pkg/pdp/types"
)

func (p *PDP) handleFindPiece(c echo.Context) error {
	ctx := c.Request().Context()

	sizeStr := c.QueryParam("size")
	if sizeStr == "" {
		return c.String(http.StatusBadRequest, "size is required")
	}
	name := c.QueryParam("name")
	if name == "" {
		return c.String(http.StatusBadRequest, "name is required")
	}
	hash := c.QueryParam("hash")
	if hash == "" {
		return c.String(http.StatusBadRequest, "hash is required")
	}

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return c.String(http.StatusBadRequest, "size is invalid")
	}

	// Verify that a 'parked_pieces' entry exists for the given 'piece_cid'
	pieceCID, has, err := p.Service.FindPiece(ctx, types.Piece{
		Name: name,
		Hash: hash,
		Size: size,
	})
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
