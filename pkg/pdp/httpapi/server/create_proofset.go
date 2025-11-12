package server

import (
	"net/http"
	"path"

	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/pdp/httpapi"
	"github.com/storacha/piri/pkg/pdp/types"
)

// echoHandleCreateProofSet -> POST /pdp/proof-sets
func (p *PDPHandler) handleCreateProofSet(c echo.Context) error {
	ctx := c.Request().Context()

	var req httpapi.CreateProofSetRequest
	if err := c.Bind(&req); err != nil {
		return types.WrapError(types.KindInvalidInput, "invalid request body", err)
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
