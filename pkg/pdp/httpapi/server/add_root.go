package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/pdp/httpapi"
	"github.com/storacha/piri/pkg/pdp/httpapi/server/middleware"
	"github.com/storacha/piri/pkg/pdp/types"
)

func (p *PDPHandler) handleAddRootToProofSet(c echo.Context) error {
	ctx := c.Request().Context()
	operation := "AddRootToProofSet"

	proofSetIDStr := c.Param("proofSetID")
	if proofSetIDStr == "" {
		return middleware.NewError(operation, "missing proofSetID", nil, http.StatusBadRequest)
	}

	id, err := strconv.ParseUint(proofSetIDStr, 10, 64)
	if err != nil {
		return middleware.NewError(operation, "invalid proofSetID format", err, http.StatusBadRequest).
			WithContext("proofSetID", proofSetIDStr)
	}

	var req httpapi.AddRootsRequest
	if err := c.Bind(&req); err != nil {
		return middleware.NewError(operation, "failed to parse request body", err, http.StatusBadRequest).
			WithContext("proofSetID", id)
	}

	if len(req.Roots) == 0 {
		return middleware.NewError(operation, "no roots provided", nil, http.StatusBadRequest).
			WithContext("proofSetID", id)
	}

	t := make([]types.RootAdd, 0, len(req.Roots))

	for _, r := range req.Roots {
		rcid, err := cid.Decode(r.RootCID)
		if err != nil {
			return middleware.NewError(operation, "invalid root CID", err, http.StatusBadRequest)
		}
		subroots := make([]cid.Cid, 0, len(r.Subroots))
		for _, s := range r.Subroots {
			scid, err := cid.Decode(s.SubrootCID)
			if err != nil {
				return middleware.NewError(operation, "invalid subroot CID", err, http.StatusBadRequest)
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
	txHash, err := p.Service.AddRoots(ctx, id, t, types.ExtraData(req.ExtraData))
	if err != nil {
		return middleware.NewError(operation, "failed to add root to proofSet", err, http.StatusInternalServerError).
			WithContext("proofSetID", id).
			WithContext("rootCount", len(req.Roots))
	}

	log.Infow("Successfully added roots to proofSet",
		"proofSetID", id,
		"rootCount", len(req.Roots),
		"duration", time.Since(start))
	return c.JSON(http.StatusCreated, httpapi.AddRootsResponse{TxHash: txHash.String()})
}
