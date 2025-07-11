package claims

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/claimstore"
)

// Handler wraps claims handler functionality for Echo
type Handler struct {
	claims claimstore.ClaimStore
}

// NewHandler creates a new claims handler
func NewHandler(claims claimstore.ClaimStore) (*Handler, error) {
	return &Handler{claims}, nil
}

// RegisterRoutes registers the claims routes with Echo
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.GET("/claim/:claim", h.handleGetClaim)
}

// handleGetClaim handles GET /claim/:claim requests
func (h *Handler) handleGetClaim(c echo.Context) error {
	claimParam := c.Param("claim")

	// Parse the CID from the claim parameter
	parsedCID, err := cid.Parse(claimParam)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid claim CID: %v", err))
	}

	// Get the claim from the store
	dlg, err := h.claims.Get(c.Request().Context(), cidlink.Link{Cid: parsedCID})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("not found: %s", parsedCID))
		}
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to get claim: %v", err))
	}

	// Copy the archive to the response
	_, err = io.Copy(c.Response(), dlg.Archive())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("serving claim: %s: %v", parsedCID, err))
	}

	return nil
}
