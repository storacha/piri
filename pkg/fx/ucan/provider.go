package ucan

import (
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
	"github.com/storacha/go-ucanto/principal"
	ucanserver "github.com/storacha/go-ucanto/server"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/fx/ucan/handlers"
	"github.com/storacha/piri/pkg/service/storage"
)

type Handler struct {
	ucanServer ucanserver.ServerView
}

var Module = fx.Module("ucan/server",
	fx.Provide(
		fx.Annotate(
			NewHandler,
			fx.ResultTags(`group:"route_registrar"`),
		),
	),
	handlers.Module,
)

var log = logging.Logger("ucan")

type Params struct {
	fx.In

	ID      principal.Signer
	Options []ucanserver.Option `group:"ucan_options"`
}

func NewHandler(p Params) (*Handler, error) {
	ucanSvr, err := ucanserver.NewServer(p.ID, p.Options...)
	if err != nil {
		return nil, fmt.Errorf("creating ucan server: %w", err)
	}

	return &Handler{ucanSvr}, nil
}

// RegisterRoutes registers the UCAN routes with Echo
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.POST("/", echo.WrapHandler(storage.NewHandler(h.ucanServer)))
}
