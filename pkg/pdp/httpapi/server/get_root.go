package server

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (p *PDP) handleGetProofSetRoot(c echo.Context) error {
	return c.NoContent(http.StatusNotImplemented)
}
