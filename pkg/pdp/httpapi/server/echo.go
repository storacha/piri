package server

import (
	"io"

	"github.com/labstack/echo/v4"
)

// handleEcho echoes back the request body
// This is useful for testing connection handling during shutdown
// Use: curl -X POST http://localhost:8080/echo --data-binary @-
// The connection will hang waiting for stdin input
func (p *PDPHandler) handleEcho(c echo.Context) error {
	// Set response headers to match request
	c.Response().Header().Set("Content-Type", c.Request().Header.Get("Content-Type"))

	// Stream the request body directly to the response
	// This will keep the connection open as long as data is being sent
	_, err := io.Copy(c.Response().Writer, c.Request().Body)
	return err
}