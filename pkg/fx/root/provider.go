package root

import (
	"github.com/labstack/echo/v4"
	"github.com/storacha/go-ucanto/principal"
	"go.uber.org/fx"

	echofx "github.com/storacha/piri/pkg/fx/echo"
	"github.com/storacha/piri/pkg/server"
)

var _ echofx.RouteRegistrar = (*Handler)(nil)

// Handler provides the root route handler
type Handler struct {
	id principal.Signer
}

// NewRootHandler creates a new root handler
func NewRootHandler(id principal.Signer) *Handler {
	return &Handler{id: id}
}

// RegisterRoutes registers the root route
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.GET("/", echo.WrapHandler(server.NewHandler(h.id)))
}

// Module provides the root handler with route registrar tag
var Module = fx.Module("root-handler",
	fx.Provide(
		fx.Annotate(
			NewRootHandler,
			fx.ResultTags(`group:"route_registrar"`),
		),
	),
)
