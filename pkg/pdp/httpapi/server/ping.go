package server

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/build"
)

func (p *PDPHandler) handlePing(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"type":    "piri",
		"version": build.Version,
	})
}
