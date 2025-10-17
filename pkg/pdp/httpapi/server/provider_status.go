package server

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/pdp/httpapi"
	"github.com/storacha/piri/pkg/pdp/httpapi/server/middleware"
)

// handleGetProviderStatus -> GET /pdp/provider/status
func (p *PDPHandler) handleGetProviderStatus(c echo.Context) error {
	ctx := c.Request().Context()
	operation := "GetProviderStatus"

	log.Debugw("Processing GetProviderStatus request")

	result, err := p.Service.GetProviderStatus(ctx)
	if err != nil {
		return middleware.NewError(operation, "Failed to get provider status", err, http.StatusInternalServerError)
	}

	resp := httpapi.GetProviderStatusResponse{
		ID:                 result.ID,
		Address:            result.Address.Hex(),
		Payee:              result.Payee.Hex(),
		IsRegistered:       result.IsRegistered,
		IsActive:           result.IsActive,
		Name:               result.Name,
		Description:        result.Description,
		RegistrationStatus: result.RegistrationStatus,
		IsApproved:         result.IsApproved,
	}

	log.Infow("Successfully processed provider status request", "isRegistered", result.IsRegistered, "status", result.RegistrationStatus)
	return c.JSON(http.StatusOK, resp)
}
