package server

import (
	"crypto/ed25519"
	"fmt"
	"path"

	"github.com/golang-jwt/jwt/v4"
	logging "github.com/ipfs/go-log/v2"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/pdp/service"
)

var log = logging.Logger("pdp/api")

const (
	PDPRoutePath     = "/pdp"
	PRoofSetRoutPath = "/proof-sets"
	PiecePrefix      = "/piece"
)

type PDPHandler struct {
	Service       *service.PDPService
	jwtMiddleware echo.MiddlewareFunc
}

func NewPDPHandler(service *service.PDPService, identity app.IdentityConfig) (*PDPHandler, error) {
	if identity.Signer == nil {
		return nil, fmt.Errorf("missing identity signer for jwt auth")
	}
	publicKey := ed25519.PublicKey(identity.Signer.Verifier().Raw())
	jwtMiddleware := echojwt.WithConfig(echojwt.Config{
		SigningKey:    publicKey,
		SigningMethod: jwt.SigningMethodEdDSA.Alg(),
	})

	return &PDPHandler{
		Service:       service,
		jwtMiddleware: jwtMiddleware,
	}, nil
}

func (p *PDPHandler) RegisterRoutes(e *echo.Echo) {
	pdpGroup := e.Group(PDPRoutePath)
	authenticated := pdpGroup.Group("", p.jwtMiddleware)

	// /pdp/proof-sets
	proofSets := authenticated.Group(PRoofSetRoutPath)
	proofSets.POST("", p.handleCreateProofSet)
	proofSets.GET("/created/:txHash", p.handleGetProofSetCreationStatus)

	// /pdp/proof-sets/:proofSetID
	proofSets.GET("/:proofSetID", p.handleGetProofSet)
	proofSets.DELETE("/:proofSetID", p.handleDeleteProofSet)
	proofSets.GET("", p.handleListProofSet)
	proofSets.GET("/:proofSetID/state", p.handleGetProofSetState)

	// /pdp/proof-sets/:proofSetID/roots
	roots := proofSets.Group("/:proofSetID/roots")
	roots.POST("", p.handleAddRootToProofSet)
	roots.GET("/:rootID", p.handleGetProofSetRoot)
	roots.DELETE("/:rootID", p.handleDeleteRootFromProofSet)

	// /pdp/ping
	pdpGroup.GET("/ping", p.handlePing)

	// /pdp/piece
	authenticated.POST(PiecePrefix, p.handlePreparePiece)
	pdpGroup.PUT(path.Join(PiecePrefix, "/upload/:uploadUUID"), p.handlePieceUpload)
	authenticated.GET(PiecePrefix, p.handleFindPiece)

	// /pdp/provider
	authenticated.POST(path.Join("/provider/register"), p.handleRegisterProvider)
	authenticated.GET(path.Join("/provider/status"), p.handleGetProviderStatus)
}
