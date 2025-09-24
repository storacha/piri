package ucan

import (
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
	"github.com/storacha/go-ucanto/principal"
	ucanserver "github.com/storacha/go-ucanto/server"
	"go.uber.org/fx"

	echofx "github.com/storacha/piri/pkg/fx/echo"
	"github.com/storacha/piri/pkg/fx/storage/ucan/handlers"
	"github.com/storacha/piri/pkg/service/storage"
)

var log = logging.Logger("fx/storage/ucan")

type Handler struct {
	ucanServer ucanserver.ServerView[ucanserver.Service]
}

var Module = fx.Module("storage/ucan/server",
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
	options := []ucanserver.Option{
		ucanserver.WithErrorHandler(func(err ucanserver.HandlerExecutionError[any]) {
			l := log.With("error", err.Error())
			if s := err.Stack(); s != "" {
				l = l.With("stack", s)
			}
			l.Error("ucan storage handler execution error")
		}),
	}
	options = append(options, p.Options...)
	ucanSvr, err := ucanserver.NewServer(p.ID, options...)
	if err != nil {
		return nil, fmt.Errorf("creating ucan server: %w", err)
	}

	return &Handler{ucanSvr}, nil
}

// RegisterRoutes registers the UCAN routes with Echo
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.POST("/", storage.NewHandler(h.ucanServer).ToEcho())
}

// AsRouteRegistrar provides the Handler as a RouteRegistrar
func AsRouteRegistrar(h *Handler) echofx.RouteRegistrar {
	return h
}

// ProvideServerView provides the UCAN ServerView for testing
func ProvideServerView(h *Handler) ucanserver.ServerView[ucanserver.Service] {
	return h.ucanServer
}
