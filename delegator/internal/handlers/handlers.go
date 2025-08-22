package handlers

import (
	"net/http"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
	"github.com/labstack/echo/v4"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/did"

	"github.com/storacha/piri/delegator/internal/service"
)

type Handlers struct {
	service *service.DelegatorService
}

func NewHandlers(svc *service.DelegatorService) *Handlers {
	return &Handlers{
		service: svc,
	}
}

func (h *Handlers) HealthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

func (h *Handlers) Root(c echo.Context) error {
	return c.String(http.StatusOK, "hello")
}

type RegisterRequest struct {
	DID           string `json:"did"`
	OwnerAddress  string `json:"owner_address"`
	ProofSetID    uint64 `json:"proof_set_id"`
	OperatorEmail string `json:"operator_email"`
	PublicURL     string `json:"public_url"`
	Proof         string `json:"proof"`
}

func (h *Handlers) Register(c echo.Context) error {
	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		return c.String(http.StatusBadRequest, "invalid request body")
	}

	// parse and validate request
	operator, err := did.Parse(req.DID)
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid DID")
	}
	if !common.IsHexAddress(req.OwnerAddress) {
		return c.String(http.StatusBadRequest, "invalid OwnerAddress")
	}
	endpoint, err := url.Parse(req.PublicURL)
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid PublicURL")
	}

	if err := h.service.Register(c.Request().Context(), service.RegisterParams{
		DID:           operator,
		OwnerAddress:  common.HexToAddress(req.OwnerAddress),
		ProofSetID:    req.ProofSetID,
		OperatorEmail: req.OperatorEmail,
		PublicURL:     *endpoint,
		Proof:         req.Proof,
	}); err != nil {
		// TODO map the errors the service returns to http codes
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.NoContent(http.StatusCreated)
}

type RequestProofRequest struct {
	DID string `json:"did"`
}

type RequestProofResponse struct {
	Proof string `json:"proof"`
}

func (h *Handlers) RequestProof(c echo.Context) error {
	var req RequestProofRequest
	if err := c.Bind(&req); err != nil {
		return c.String(http.StatusBadRequest, "invalid request body")
	}

	operator, err := did.Parse(req.DID)
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid DID")
	}

	resp, err := h.service.RequestProof(c.Request().Context(), operator)
	if err != nil {
		// TODO map the errors the service returns to http codes
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	proof, err := delegation.Format(resp)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to read generated proof")
	}

	return c.JSON(http.StatusOK, RequestProofResponse{Proof: proof})
}

type IsRegisteredRequest struct {
	DID string `json:"did"`
}

func (h *Handlers) IsRegistered(c echo.Context) error {
	var req IsRegisteredRequest
	if err := c.Bind(&req); err != nil {
		return c.String(http.StatusBadRequest, "invalid request body")
	}

	operator, err := did.Parse(req.DID)
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid DID")
	}

	registered, err := h.service.IsRegisteredDID(c.Request().Context(), operator)
	if err != nil {
		// TODO map the errors the service returns to http codes
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	if registered {
		return c.NoContent(http.StatusOK)
	}

	return c.NoContent(http.StatusNotFound)
}
