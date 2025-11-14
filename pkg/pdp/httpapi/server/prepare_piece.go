package server

import (
	"net/http"
	"net/url"
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/labstack/echo/v4"
	"github.com/multiformats/go-multihash"

	"github.com/storacha/piri/pkg/pdp/httpapi"
	"github.com/storacha/piri/pkg/pdp/proof"
	"github.com/storacha/piri/pkg/pdp/types"
)

var PieceSizeLimit = abi.PaddedPieceSize(proof.MaxMemtreeSize).Unpadded()

// handlePreparePiece -> POST /pdp/piece
func (p *PDPHandler) handlePreparePiece(c echo.Context) error {
	ctx := c.Request().Context()

	var req httpapi.AddPieceRequest
	if err := c.Bind(&req); err != nil {
		return types.WrapError(types.KindInvalidInput, "invalid request body", err)
	}

	if abi.UnpaddedPieceSize(req.Check.Size) > PieceSizeLimit {
		return types.NewErrorf(types.KindInvalidInput, "piece size too large. expected: %d actual: %d", PieceSizeLimit, req.Check.Size)
	}

	log.Debugw("Processing prepare piece request",
		"name", req.Check,
		"hash", req.Check.Hash,
		"size", req.Check.Size)
	start := time.Now()
	params := types.PieceAllocation{
		Piece: types.Piece{
			Name: req.Check.Name,
			Hash: multihash.Multihash(req.Check.Hash),
			Size: req.Check.Size,
		},
	}
	if req.Notify != "" {
		var err error
		params.Notify, err = url.Parse(req.Notify)
		if err != nil {
			return types.WrapError(types.KindInvalidInput, "invalid notify url", err)
		}
	}
	res, err := p.Service.AllocatePiece(ctx, params)
	if err != nil {
		return err
	}

	resp := httpapi.AddPieceResponse{
		Allocated: res.Allocated,
		PieceCID:  res.Piece.String(),
		UploadID:  res.UploadID.String(),
	}
	log.Infow("Successfully prepared piece",
		"uploadID", resp.UploadID,
		"allocated", resp.Allocated,
		"duration", time.Since(start))
	if res.Allocated {
		return c.JSON(http.StatusCreated, resp)
	}
	return c.JSON(http.StatusOK, resp)
}
