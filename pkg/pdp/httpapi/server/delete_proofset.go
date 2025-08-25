package server

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/pdp/httpapi"
)

// echoHandleDeleteProofSet -> DELETE /pdp/proof-sets/:proofSetID
func (p *PDPHandler) handleDeleteProofSet(c echo.Context) error {
	ctx := c.Request().Context()
	
	// Extract parameters from the URL
	proofSetIDStr := c.Param("proofSetID")
	if proofSetIDStr == "" {
		return c.String(http.StatusBadRequest, "missing proofSetID")
	}

	proofSetID, err := strconv.ParseUint(proofSetIDStr, 10, 64)
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid proofSetID")
	}

	// Call the service to delete the proof set
	txHash, err := p.Service.DeleteProofSet(ctx, proofSetID)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to delete proof set")
	}
	
	return c.JSON(http.StatusOK, httpapi.DeleteProofSetResponse{TxHash: txHash.String()})
}
