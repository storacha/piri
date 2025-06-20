package http

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/labstack/echo/v4"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/claimstore"
)

var log = logging.Logger("claim-server")

// Claim provides HTTP endpoints for claims
type Claim struct {
	claimStore claimstore.ClaimStore
}

// Params defines the dependencies for the server
type Params struct {
	fx.In
	ClaimStore claimstore.ClaimStore
}

// NewClaimHandler creates a new claims server
func NewClaimHandler(params Params) *Claim {
	return &Claim{
		claimStore: params.ClaimStore,
	}
}

// RegisterRoutes registers all routes for the claims server
func (s *Claim) RegisterRoutes(e *echo.Echo) {
	// Create a group for claim routes
	group := e.Group("/claim")
	group.GET("/:claim", s.GetClaim)
}

// GetClaim handles GET /claim/{claim}
func (s *Claim) GetClaim(c echo.Context) error {
	claimParam := c.Param("claim")

	// Try to parse as CID first
	var claimCID cid.Cid
	claimCID, err := cid.Parse(claimParam)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid claim CID: %s", claimParam))
	}

	// Get the claim from the store
	dlg, err := s.claimStore.Get(c.Request().Context(), cidlink.Link{Cid: claimCID})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("claim '%s' not found", claimParam))
		}
		log.Errorf("failed to get claim %s: %v", claimCID, err)
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to get claim %s: %v", claimCID, err))
	}

	// NB: previous behavior, seems poorly defined
	/*
		// Get claim archive (CAR format)
		_, err = io.Copy(c.Response(), dlg.Archive())
		if err != nil {
			return fmt.Errorf("serving claim: %s: %w", c, err)
		}
	*/

	// Write the archive data
	respPayload, err := io.ReadAll(dlg.Archive())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to read response payload for claim %s: %v", claimCID, err))
	}
	return c.Blob(http.StatusOK, "application/vnd.ipld.car", respPayload)
}
