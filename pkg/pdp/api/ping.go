package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (p *PDP) handlePing(c echo.Context) error {
	return c.NoContent(http.StatusOK)
}
