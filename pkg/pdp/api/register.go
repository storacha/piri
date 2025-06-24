package api

import (
	"path"

	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/pdp/api/middleware"
	"github.com/storacha/piri/pkg/pdp/service"
)

var log = logging.Logger("pdp/api")

const (
	PDPRoutePath     = "/pdp"
	PRoofSetRoutPath = "/proof-sets"
	PiecePrefix      = "/piece"
)

func RegisterEchoRoutes(e *echo.Echo, p *PDP) {
	// Apply authentication middleware to protected routes if configured
	var authMiddleware echo.MiddlewareFunc
	if p.AuthConfig != nil {
		if p.AuthConfig.Required {
			authMiddleware = middleware.RequiredAuth(p.AuthConfig.ServiceName, p.AuthConfig.TrustedPrincipals...)
		} else {
			authMiddleware = middleware.OptionalAuth(p.AuthConfig.ServiceName, p.AuthConfig.TrustedPrincipals...)
		}
	}

	// Public routes (no authentication required)
	// /pdp/ping - health check
	e.GET("/pdp/ping", p.handlePing)
	
	// /piece/:cid - piece retrieval (kept public for performance)
	e.GET(path.Join(PiecePrefix, ":cid"), p.handleDownloadByPieceCid)

	// Protected routes (require authentication)
	// /pdp/proof-sets
	proofSets := e.Group(path.Join(PDPRoutePath, PRoofSetRoutPath))
	if authMiddleware != nil {
		proofSets.Use(authMiddleware)
	}
	proofSets.POST("", p.handleCreateProofSet)
	proofSets.GET("/created/:txHash", p.handleGetProofSetCreationStatus)

	// /pdp/proof-sets/:proofSetID
	proofSets.GET("/:proofSetID", p.handleGetProofSet)
	proofSets.DELETE("/:proofSetID", p.handleDeleteProofSet)

	// /pdp/proof-sets/:proofSetID/roots
	roots := proofSets.Group("/:proofSetID/roots")
	roots.POST("", p.handleAddRootToProofSet)
	roots.GET("/:rootID", p.handleGetProofSetRoot)
	roots.DELETE("/:rootID", p.handleDeleteRootFromProofSet)

	// Protected piece management routes
	pieceGroup := e.Group(path.Join(PDPRoutePath, PiecePrefix))
	if authMiddleware != nil {
		pieceGroup.Use(authMiddleware)
	}
	pieceGroup.POST("", p.handlePreparePiece)
	pieceGroup.PUT("/upload/:uploadUUID", p.handlePieceUpload)
	pieceGroup.GET("", p.handleFindPiece)
}

type PDP struct {
	Service *service.PDPService
	// Authentication configuration
	AuthConfig *AuthConfig
}

// AuthConfig configures authentication for the PDP API
type AuthConfig struct {
	// ServiceName is the expected service name in JWT tokens
	ServiceName string
	// TrustedPrincipals is a list of trusted principal DIDs. If empty, any valid JWT is accepted
	TrustedPrincipals []string
	// Required indicates whether authentication is required for protected endpoints
	Required bool
}
