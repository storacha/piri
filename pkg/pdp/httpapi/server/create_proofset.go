package server

import (
	"net/http"
	"path"

	"github.com/ethereum/go-ethereum/common"
	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/pdp/httpapi/server/middleware"
	"github.com/storacha/piri/pkg/pdp/types"

	"github.com/storacha/piri/pkg/pdp/httpapi"
)

// echoHandleCreateProofSet -> POST /pdp/proof-sets
func (p *PDPHandler) handleCreateProofSet(c echo.Context) error {
	ctx := c.Request().Context()
	operation := "CreateProofSet"

	var req httpapi.CreateProofSetRequest
	if err := c.Bind(&req); err != nil {
		return middleware.NewError(operation, "Invalid request body", err, http.StatusBadRequest)
	}
	if req.RecordKeeper == "" {
		return middleware.NewError(operation, "Record Keeper is required", nil, http.StatusBadRequest)
	}
	recordKeeperAddr := common.HexToAddress(req.RecordKeeper)
	if recordKeeperAddr == (common.Address{}) {
		return middleware.NewError(operation, "Record Keeper is invalid", nil, http.StatusBadRequest).
			WithContext("address (invalid)", req.RecordKeeper)
	}

	log.Debugw("Processing CreateProofSet request", "recordKeeper", recordKeeperAddr)

	txHash, err := p.Service.CreateProofSet(ctx, types.CreateProofSetParams{
		RecordKeeper: recordKeeperAddr,
		ExtraData:    types.ExtraData(req.ExtraData),
	})
	if err != nil {
		return middleware.NewError(operation, "Failed to create proof set", err, http.StatusInternalServerError)
	}

	location := path.Join("/pdp/proof-sets/created", txHash.Hex())
	c.Response().Header().Set("Location", location)

	resp := httpapi.CreateProofSetResponse{
		TxHash:   txHash.Hex(),
		Location: location,
	}
	log.Infow("Successfully initiated proof set creation", "txHash", txHash.Hex(), "location", location)
	return c.JSON(http.StatusCreated, resp)
}
