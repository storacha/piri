package server

import (
	"path"

	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
	"go.uber.org/fx"

	echofx "github.com/storacha/piri/pkg/fx/echo"
	"github.com/storacha/piri/pkg/pdp/service"
)

var Module = fx.Module("pdp/server",
	fx.Provide(
		fx.Annotate(
			NewPDPServer,
			fx.As(new(echofx.RouteRegistrar)),
			fx.ResultTags(`group:"route_registrar"`),
		),
	),
)

var log = logging.Logger("pdp/api")

const (
	PDPRoutePath     = "/pdp"
	PRoofSetRoutPath = "/proof-sets"
	PiecePrefix      = "/piece"
)

func NewPDPServer(service *service.PDPService) *PDP {
	return &PDP{
		service,
	}
}

type PDP struct {
	Service *service.PDPService
}

func (p *PDP) RegisterRoutes(e *echo.Echo) {
	// /pdp/proof-sets
	proofSets := e.Group(path.Join(PDPRoutePath, PRoofSetRoutPath))
	proofSets.POST("", p.handleCreateProofSet)
	proofSets.GET("/created/:txHash", p.handleGetProofSetCreationStatus)

	proofSets.GET("", p.handleListProofSet)
	// /pdp/proof-sets/:proofSetID
	proofSets.GET("/:proofSetID", p.handleGetProofSet)
	proofSets.DELETE("/:proofSetID", p.handleDeleteProofSet)

	// /pdp/proof-sets/:proofSetID/roots
	roots := proofSets.Group("/:proofSetID/roots")
	roots.POST("", p.handleAddRootToProofSet)
	roots.GET("/:rootID", p.handleGetProofSetRoot)
	roots.DELETE("/:rootID", p.handleDeleteRootFromProofSet)

	// /pdp/ping
	e.GET("/pdp/ping", p.handlePing)

	// /pdp/piece
	e.POST(path.Join(PDPRoutePath, piecePrefix), p.handlePreparePiece)
	e.PUT(path.Join(PDPRoutePath, piecePrefix, "/upload/:uploadUUID"), p.handlePieceUpload)
	e.GET(path.Join(PDPRoutePath, piecePrefix), p.handleFindPiece)

	// retrival
	e.GET(path.Join(PiecePrefix, ":cid"), p.handleDownloadByPieceCid)
}
