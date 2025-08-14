package ucan

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/storacha/go-ucanto/principal"
	ucanserver "github.com/storacha/go-ucanto/server"
	"go.uber.org/fx"

	echofx "github.com/storacha/piri/pkg/fx/echo"
	"github.com/storacha/piri/pkg/fx/ucan/handlers"
	"github.com/storacha/piri/pkg/service/storage"
)

type Handler struct {
	ucanServer ucanserver.ServerView
}

var Module = fx.Module("ucan/server",
	fx.Provide(
		NewHandler,
		fx.Annotate(
			AsRouteRegistrar,
			fx.ResultTags(`group:"route_registrar"`),
		),
		ProvideServerView,
	),
	handlers.Module,
)

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

// AsRouteRegistrar provides the Handler as a RouteRegistrar
func AsRouteRegistrar(h *Handler) echofx.RouteRegistrar {
	return h
}

// ProvideServerView provides the UCAN ServerView for testing
func ProvideServerView(h *Handler) ucanserver.ServerView {
	return h.ucanServer
}
