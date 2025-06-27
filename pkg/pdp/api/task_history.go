package api

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/pdp/api/middleware"
	"github.com/storacha/piri/pkg/pdp/types"
)

func (p *PDP) handleGetTaskHistory(c echo.Context) error {
	ctx := c.Request().Context()
	operations := "GetTaskHistory"

	// Parse filter from query parameters
	filter, err := types.NewTaskHistoryFilterFromQuery(c.QueryParams())
	if err != nil {
		return middleware.NewError(
			operations,
			"invalid filter parameters",
			err,
			http.StatusBadRequest,
		)
	}
	
	// Validate filter
	if err := filter.Validate(); err != nil {
		return middleware.NewError(
			operations,
			"invalid filter values",
			err,
			http.StatusBadRequest,
		)
	}

	history, err := p.Service.GetTaskHistory(ctx, filter)
	if err != nil {
		return middleware.NewError(
			operations,
			"failed to read task history",
			err,
			http.StatusInternalServerError,
		)
	}

	return c.JSON(http.StatusOK, history)
}
