package server

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/pdp/httpapi"
)

// handleRepairProofSet -> POST /pdp/proof-sets/:proofSetID/repair
func (p *PDPHandler) handleRepairProofSet(c echo.Context) error {
	ctx := c.Request().Context()
	proofSetIDStr := c.Param("proofSetID")

	if proofSetIDStr == "" {
		return c.String(http.StatusBadRequest, "missing proofSetID")
	}

	id, err := strconv.ParseUint(proofSetIDStr, 10, 64)
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid proofSetID")
	}

	result, err := p.Service.RepairProofSet(ctx, id)
	if err != nil {
		log.Errorw("failed to repair proof set", "proofSetID", id, "error", err)
		return c.String(http.StatusInternalServerError, "failed to repair proof set: "+err.Error())
	}

	resp := httpapi.RepairProofSetResponse{
		TotalOnChain:      result.TotalOnChain,
		TotalInDB:         result.TotalInDB,
		TotalRepaired:     result.TotalRepaired,
		TotalUnrepaired:   result.TotalUnrepaired,
		RepairedEntries:   make([]httpapi.RepairedEntry, len(result.RepairedEntries)),
		UnrepairedEntries: make([]httpapi.UnrepairedEntry, len(result.UnrepairedEntries)),
	}

	for i, entry := range result.RepairedEntries {
		resp.RepairedEntries[i] = httpapi.RepairedEntry{
			RootCID:  entry.RootCID,
			RootID:   entry.RootID,
			Subroots: entry.Subroots,
		}
	}

	for i, entry := range result.UnrepairedEntries {
		resp.UnrepairedEntries[i] = httpapi.UnrepairedEntry{
			RootCID: entry.RootCID,
			RootID:  entry.RootID,
			Reason:  entry.Reason,
		}
	}

	return c.JSON(http.StatusOK, resp)
}
