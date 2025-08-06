package server

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/pdp/httpapi"
)

// handleGetProofSet -> GET /pdp/proof-sets/:proofSetID
func (p *PDP) handleGetProofSet(c echo.Context) error {
	ctx := c.Request().Context()
	proofSetIDStr := c.Param("proofSetID")

	if proofSetIDStr == "" {
		return c.String(http.StatusBadRequest, "missing proofSetID")
	}

	id, err := strconv.ParseUint(proofSetIDStr, 10, 64)
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid proofSetID")
	}

	ps, err := p.Service.GetProofSet(ctx, id)
	if err != nil {
		return err
	}

	resp := httpapi.GetProofSetResponse{
		ID:                 ps.ID,
		NextChallengeEpoch: &ps.NextChallengeEpoch,
	}
	for _, root := range ps.Roots {
		resp.Roots = append(resp.Roots, httpapi.RootEntry{
			RootID:        root.RootID,
			RootCID:       root.RootCID.String(),
			SubrootCID:    root.SubrootCID.String(),
			SubrootOffset: root.SubrootOffset,
		})
	}
	return c.JSON(http.StatusOK, resp)
}
