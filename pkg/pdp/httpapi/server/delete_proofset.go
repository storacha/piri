package server

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// echoHandleDeleteProofSet -> DELETE /pdp/proof-sets/:proofSetID
func (p *PDPHandler) handleDeleteProofSet(c echo.Context) error {
	proofSetIDStr := c.Param("proofSetID")
	_, _ = strconv.ParseUint(proofSetIDStr, 10, 64) // ignoring error for brevity

	// TODO: plug in your logic

	return c.NoContent(http.StatusNotImplemented)
}
