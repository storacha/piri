package server

import (
	"encoding/hex"
	"net/http"
	"strconv"

	"github.com/ipfs/go-cid"
	"github.com/labstack/echo/v4"
	"github.com/multiformats/go-multihash"

	"github.com/storacha/piri/pkg/pdp/httpapi"
	"github.com/storacha/piri/pkg/pdp/types"
)

func (p *PDPHandler) handleFindPiece(c echo.Context) error {
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
	mh, err := Multihash(types.Piece{
		Name: name,
		Hash: hash,
		Size: size,
	})
	if err != nil {
		return c.String(http.StatusBadRequest, "hash is invalid")
	}
	dmh, err := multihash.Decode(mh)
	if err != nil {
		return c.String(http.StatusBadRequest, "hash is invalid")
	}
	toResolve := cid.NewCidV1(dmh.Code, dmh.Digest)
	pieceCID, has, err := p.Service.ResolvePiece(ctx, toResolve)
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

func Multihash(piece types.Piece) (multihash.Multihash, error) {
	_, ok := multihash.Names[piece.Name]
	if !ok {
		return nil, types.NewErrorf(types.KindInvalidInput, "unknown multihash type: %s", piece.Name)
	}

	hashBytes, err := hex.DecodeString(piece.Hash)
	if err != nil {
		return nil, types.WrapError(types.KindInvalidInput, "failed to decode hash", err)
	}

	return multihash.EncodeName(hashBytes, piece.Name)
}
