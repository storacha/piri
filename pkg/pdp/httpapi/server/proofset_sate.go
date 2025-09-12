package server

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/pdp/httpapi"
)

// handleListProofSet -> GET /pdp/

// handleGetProofSet -> GET /pdp/proof-sets/:proofSetID
func (p *PDPHandler) handleGetProofSetState(c echo.Context) error {
	ctx := c.Request().Context()
	proofSetIDStr := c.Param("proofSetID")

	if proofSetIDStr == "" {
		return c.String(http.StatusBadRequest, "missing proofSetID")
	}

	id, err := strconv.ParseUint(proofSetIDStr, 10, 64)
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid proofSetID")
	}

	ps, err := p.Service.GetProofSetState(ctx, id)
	if err != nil {
		return err
	}
	cs := ps.ContractState

	resp := httpapi.GetProofSetStateResponse{
		ID:                     ps.ID,
		Initialized:            ps.Initialized,
		NextChallengeEpoch:     ps.NextChallengeEpoch,
		PreviousChallengeEpoch: ps.PreviousChallengeEpoch,
		ProvingPeriod:          ps.ProvingPeriod,
		ChallengeWindow:        ps.ChallengeWindow,
		CurrentEpoch:           ps.CurrentEpoch,
		ChallengedIssued:       ps.ChallengedIssued,
		InChallengeWindow:      ps.InChallengeWindow,
		IsInFaultState:         ps.IsInFaultState,
		HasProven:              ps.HasProven,
		IsProving:              ps.IsProving,
		ContractState: httpapi.ProofSetContractState{
			Owners:                   cs.Owners,
			NextChallengeWindowStart: cs.NextChallengeWindowStart,
			NextChallengeEpoch:       cs.NextChallengeEpoch,
			MaxProvingPeriod:         cs.MaxProvingPeriod,
			ChallengeWindow:          cs.ChallengeWindow,
			ChallengeRange:           cs.ChallengeRange,
			ScheduledRemovals:        cs.ScheduledRemovals,
			ProofFee:                 cs.ProofFee,
			ProofFeeBuffered:         cs.ProofFeeBuffered,
		},
	}
	return c.JSON(http.StatusOK, resp)
}
