package server

import (
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/pdp/httpapi"
)

// echoHandleGetProofSetCreationStatus -> GET /pdp/proof-sets/created/:txHash
func (p *PDPHandler) handleGetProofSetCreationStatus(c echo.Context) error {
	ctx := c.Request().Context()
	txHash := c.Param("txHash")

	// Clean txHash (ensure it starts with '0x' and is lowercase)
	if !strings.HasPrefix(txHash, "0x") {
		txHash = "0x" + txHash
	}
	txHash = strings.ToLower(txHash)

	// Validate txHash is a valid hash
	if len(txHash) != 66 { // '0x' + 64 hex chars
		return c.String(http.StatusBadRequest, "Invalid txHash length")
	}
	if _, err := hex.DecodeString(txHash[2:]); err != nil {
		return c.String(http.StatusBadRequest, "Invalid txHash format")
	}
	txh := common.HexToHash(txHash)

	status, err := p.Service.GetProofSetStatus(ctx, txh)
	if err != nil {
		log.Errorw("failed to get status proof set creation", "error", err)
		return err
	}

	resp := httpapi.ProofSetStatusResponse{
		CreateMessageHash: status.TxHash.String(),
		ProofsetCreated:   status.Created,
		Service:           "storacha",
		TxStatus:          status.TxStatus,
		OK:                nil,
		ProofSetId:        &status.ID,
	}
	return c.JSON(http.StatusOK, resp)

}
