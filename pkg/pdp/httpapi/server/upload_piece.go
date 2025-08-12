package server

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/pdp/types"
)

func (p *PDPHandler) handlePieceUpload(c echo.Context) error {
	ctx := c.Request().Context()
	uploadUUID := c.Param("uploadUUID")

	if uploadUUID == "" {
		return c.String(http.StatusBadRequest, "uploadUUID is required")
	}

	uploadID, err := uuid.Parse(uploadUUID)
	if err != nil {
		return c.String(http.StatusBadRequest, "uploadUUID is invalid")
	}

	log.Debugw("Processing prepare piece request", "uploadID", uploadID)
	start := time.Now()
	if err := p.Service.UploadPiece(ctx, types.PieceUpload{
		ID:   uploadID,
		Data: c.Request().Body,
	}); err != nil {
		return c.String(http.StatusBadRequest, "Failed to upload piece")
	}

	log.Infow("Successfully uploaded piece",
		"uploadID", uploadID,
		"duration", time.Since(start))

	return c.NoContent(http.StatusNoContent)
}
