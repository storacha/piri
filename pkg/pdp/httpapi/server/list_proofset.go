package server

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/pdp/httpapi"
)

// handleListProofSet -> GET /pdp/proof-sets
func (p *PDPHandler) handleListProofSet(c echo.Context) error {
	ctx := c.Request().Context()

	ps, err := p.Service.ListProofSets(ctx)
	if err != nil {
		return err
	}

	var resp httpapi.ListProofSetsResponse
	for _, p := range ps {
		entry := httpapi.ProofSetEntry{
			ID:                     p.ID,
			Initialized:            p.Initialized,
			NextChallengeEpoch:     &p.NextChallengeEpoch,
			PreviousChallengeEpoch: &p.PreviousChallengeEpoch,
			ProvingPeriod:          &p.ProvingPeriod,
			ChallengeWindow:        &p.ChallengeWindow,
		}
		for _, root := range p.Roots {
			entry.Roots = append(entry.Roots, httpapi.RootEntry{
				RootID:        root.RootID,
				RootCID:       root.RootCID.String(),
				SubrootCID:    root.SubrootCID.String(),
				SubrootOffset: root.SubrootOffset,
			})
		}
		resp = append(resp, entry)
	}
	return c.JSON(http.StatusOK, resp)
}
