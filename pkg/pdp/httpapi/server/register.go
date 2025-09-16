package server

import (
	"path"

	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/pdp/service"
)

var log = logging.Logger("pdp/api")

const (
	PDPRoutePath     = "/pdp"
	PRoofSetRoutPath = "/proof-sets"
	PiecePrefix      = "/piece"
)

func NewPDPHandler(service *service.PDPService) *PDPHandler {
	return &PDPHandler{
		Service: service,
	}
}

type PDPHandler struct {
	Service *service.PDPService
}

func (p *PDPHandler) RegisterRoutes(e *echo.Echo) {
	// /pdp/proof-sets
	proofSets := e.Group(path.Join(PDPRoutePath, PRoofSetRoutPath))
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
	e.GET("/pdp/ping", p.handlePing)

	// /echo - test endpoint for connection handling
	e.POST("/echo", p.handleEcho)

	// /pdp/piece
	e.POST(path.Join(PDPRoutePath, piecePrefix), p.handlePreparePiece)
	e.PUT(path.Join(PDPRoutePath, piecePrefix, "/upload/:uploadUUID"), p.handlePieceUpload)
	e.GET(path.Join(PDPRoutePath, piecePrefix), p.handleFindPiece)

	// retrieval, supporting head requests too
	e.GET(path.Join(PiecePrefix, ":cid"), p.handleDownloadByPieceCid)
	e.HEAD(path.Join(PiecePrefix, ":cid"), p.handleDownloadByPieceCid)
}
