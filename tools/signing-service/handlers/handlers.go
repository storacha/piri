package handlers

import (
	"fmt"
	"math/big"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/labstack/echo/v4"
	"github.com/storacha/piri/tools/service-operator/eip712"
	"github.com/storacha/piri/tools/signing-service/types"
)

// Handler wraps the EIP-712 signer and provides HTTP endpoints
type Handler struct {
	signer *eip712.Signer
}

// NewHandler creates a new HTTP handler
func NewHandler(signer *eip712.Signer) *Handler {
	return &Handler{signer: signer}
}

// Health returns service health status
func (h *Handler) Health(c echo.Context) error {
	response := types.HealthResponse{
		Status: "healthy",
		Signer: h.signer.GetAddress().Hex(),
	}
	return c.JSON(http.StatusOK, response)
}

// SignCreateDataSet handles POST /sign/create-dataset
func (h *Handler) SignCreateDataSet(c echo.Context) error {
	var req types.CreateDataSetRequest
	if err := c.Bind(&req); err != nil {
		c.Logger().Errorf("Error decoding request: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request: %v", err))
	}

	// Parse clientDataSetId
	clientDataSetId, ok := new(big.Int).SetString(req.ClientDataSetId, 10)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid clientDataSetId")
	}

	// Parse payee address
	if !common.IsHexAddress(req.Payee) {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid payee address")
	}
	payee := common.HexToAddress(req.Payee)

	// Sign the data
	signature, err := h.signer.SignCreateDataSet(clientDataSetId, payee, req.Metadata)
	if err != nil {
		c.Logger().Errorf("Error signing CreateDataSet: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Signing error: %v", err))
	}

	// Verify signature by recovering signer
	recoveredSigner, err := h.signer.RecoverCreateDataSetSigner(clientDataSetId, payee, req.Metadata, signature)
	if err != nil {
		c.Logger().Errorf("Failed to recover signer: %v", err)
	} else {
		c.Logger().Infof("Signature verification - Expected: %s, Recovered: %s, Match: %v",
			signature.Signer.Hex(), recoveredSigner.Hex(), signature.Signer == recoveredSigner)
	}

	c.Logger().Infof("Signed CreateDataSet - datasetId=%s, payee=%s, signer=%s",
		req.ClientDataSetId, req.Payee, signature.Signer.Hex())

	return c.JSON(http.StatusOK, signature)
}

// SignAddPieces handles POST /sign/add-pieces
func (h *Handler) SignAddPieces(c echo.Context) error {
	var req types.AddPiecesRequest
	if err := c.Bind(&req); err != nil {
		c.Logger().Errorf("Error decoding request: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request: %v", err))
	}

	// Parse clientDataSetId
	clientDataSetId, ok := new(big.Int).SetString(req.ClientDataSetId, 10)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid clientDataSetId")
	}

	// Parse firstAdded
	firstAdded, ok := new(big.Int).SetString(req.FirstAdded, 10)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid firstAdded")
	}

	// Parse piece data (convert hex strings to bytes)
	pieceData := make([][]byte, len(req.PieceData))
	for i, hexData := range req.PieceData {
		pieceData[i] = common.FromHex(hexData)
	}

	// Sign the data
	signature, err := h.signer.SignAddPieces(clientDataSetId, firstAdded, pieceData, req.Metadata)
	if err != nil {
		c.Logger().Errorf("Error signing AddPieces: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Signing error: %v", err))
	}

	c.Logger().Infof("Signed AddPieces - datasetId=%s, pieces=%d, signer=%s",
		req.ClientDataSetId, len(req.PieceData), signature.Signer.Hex())

	return c.JSON(http.StatusOK, signature)
}

// SignSchedulePieceRemovals handles POST /sign/schedule-piece-removals
func (h *Handler) SignSchedulePieceRemovals(c echo.Context) error {
	var req types.SchedulePieceRemovalsRequest
	if err := c.Bind(&req); err != nil {
		c.Logger().Errorf("Error decoding request: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request: %v", err))
	}

	// Parse clientDataSetId
	clientDataSetId, ok := new(big.Int).SetString(req.ClientDataSetId, 10)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid clientDataSetId")
	}

	// Parse piece IDs
	pieceIds := make([]*big.Int, len(req.PieceIds))
	for i, idStr := range req.PieceIds {
		id, ok := new(big.Int).SetString(idStr, 10)
		if !ok {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid pieceId at index %d", i))
		}
		pieceIds[i] = id
	}

	// Sign the data
	signature, err := h.signer.SignSchedulePieceRemovals(clientDataSetId, pieceIds)
	if err != nil {
		c.Logger().Errorf("Error signing SchedulePieceRemovals: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Signing error: %v", err))
	}

	c.Logger().Infof("Signed SchedulePieceRemovals - datasetId=%s, pieces=%d, signer=%s",
		req.ClientDataSetId, len(req.PieceIds), signature.Signer.Hex())

	return c.JSON(http.StatusOK, signature)
}

// SignDeleteDataSet handles POST /sign/delete-dataset
func (h *Handler) SignDeleteDataSet(c echo.Context) error {
	var req types.DeleteDataSetRequest
	if err := c.Bind(&req); err != nil {
		c.Logger().Errorf("Error decoding request: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request: %v", err))
	}

	// Parse clientDataSetId
	clientDataSetId, ok := new(big.Int).SetString(req.ClientDataSetId, 10)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid clientDataSetId")
	}

	// Sign the data
	signature, err := h.signer.SignDeleteDataSet(clientDataSetId)
	if err != nil {
		c.Logger().Errorf("Error signing DeleteDataSet: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Signing error: %v", err))
	}

	c.Logger().Infof("Signed DeleteDataSet - datasetId=%s, signer=%s",
		req.ClientDataSetId, signature.Signer.Hex())

	return c.JSON(http.StatusOK, signature)
}
