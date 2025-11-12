package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/pdp/httpapi"
	"github.com/storacha/piri/pkg/pdp/types"
)

func (p *PDPHandler) handleAddRootToProofSet(c echo.Context) error {
	ctx := c.Request().Context()

	proofSetIDStr := c.Param("proofSetID")
	if proofSetIDStr == "" {
		return types.NewError(types.KindInvalidInput, "proofSetID required")
	}

	id, err := strconv.ParseUint(proofSetIDStr, 10, 64)
	if err != nil {
		return types.WrapError(types.KindInvalidInput, "invalid proofset id", err)
	}

	var req httpapi.AddRootsRequest
	if err := c.Bind(&req); err != nil {
		return types.WrapError(types.KindInvalidInput, "failed to bind request", err)
	}

	if len(req.Roots) == 0 {
		return types.NewError(types.KindInvalidInput, "no roots provided")
	}

	t := make([]types.RootAdd, 0, len(req.Roots))

	for _, r := range req.Roots {
		rcid, err := cid.Decode(r.RootCID)
		if err != nil {
			return types.WrapError(types.KindInvalidInput, "invalid root cid", err)
		}
		subroots := make([]cid.Cid, 0, len(r.Subroots))
		for _, s := range r.Subroots {
			scid, err := cid.Decode(s.SubrootCID)
			if err != nil {
				return types.WrapError(types.KindInvalidInput, "invalid subroot cid", err)
			}
			subroots = append(subroots, scid)
		}
		t = append(t, types.RootAdd{
			Root:     rcid,
			SubRoots: subroots,
		})
	}

	log.Debugw("Processing add root request",
		"proofSetID", id,
		"rootCount", len(req.Roots))

	start := time.Now()
	txHash, err := p.Service.AddRoots(ctx, id, t)
	if err != nil {
		return err
	}

	log.Infow("Successfully added roots to proofSet",
		"proofSetID", id,
		"rootCount", len(req.Roots),
		"duration", time.Since(start))
	return c.JSON(http.StatusCreated, httpapi.AddRootsResponse{TxHash: txHash.String()})
}
