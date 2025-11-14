package server

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/pdp/httpapi"
	"github.com/storacha/piri/pkg/pdp/types"
)

// handleRegisterProvider -> POST /pdp/provider
func (p *PDPHandler) handleRegisterProvider(c echo.Context) error {
	ctx := c.Request().Context()

	var req httpapi.RegisterProviderRequest
	if err := c.Bind(&req); err != nil {
		return types.WrapError(types.KindInvalidInput, "invalid request body", err)
	}

	log.Debugw("Processing RegisterProvider request", "name", req.Name, "description", req.Description)

	result, err := p.Service.RegisterProvider(ctx, types.RegisterProviderParams{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		return err
	}

	resp := httpapi.RegisterProviderResponse{
		TxHash:      result.TransactionHash.Hex(),
		Address:     result.Address.Hex(),
		Payee:       result.Payee.Hex(),
		ID:          result.ID,
		IsActive:    result.IsActive,
		Name:        result.Name,
		Description: result.Description,
	}

	log.Infow("Successfully processed provider registration", "txHash", result.TransactionHash.Hex(), "providerId", result.ID)
	return c.JSON(http.StatusCreated, resp)
}
