package server

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/pdp/httpapi"
)

type GetProofSetResponse struct {
	ID                 int64       `json:"id"`
	NextChallengeEpoch int64       `json:"nextChallengeEpoch"`
	Roots              []RootEntry `json:"roots"`
}

type RootEntry struct {
	RootID        int64  `json:"rootId"`
	RootCID       string `json:"rootCid"`
	SubrootCID    string `json:"subrootCid"`
	SubrootOffset int64  `json:"subrootOffset"`
}

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
		return c.String(http.StatusInternalServerError, "failed to fetch proofSet")
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
