package server

import (
	"net/http"
	"path"

	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/pdp/httpapi"
	"github.com/storacha/piri/pkg/pdp/httpapi/server/middleware"
)

// echoHandleCreateProofSet -> POST /pdp/proof-sets
func (p *PDPHandler) handleCreateProofSet(c echo.Context) error {
	ctx := c.Request().Context()
	operation := "CreateProofSet"

	var req httpapi.CreateProofSetRequest
	if err := c.Bind(&req); err != nil {
		return middleware.NewError(operation, "Invalid request body", err, http.StatusBadRequest)
	}

	txHash, err := p.Service.CreateProofSet(ctx)
	if err != nil {
		return err
	}

	location := path.Join("/pdp/proof-sets/created", txHash.Hex())
	c.Response().Header().Set("Location", location)

	resp := httpapi.CreateProofSetResponse{
		TxHash:   txHash.Hex(),
		Location: location,
	}
	log.Infow("Successfully initiated proof set creation", "txHash", txHash.Hex(), "location", location)
	return c.JSON(http.StatusCreated, resp)
}
