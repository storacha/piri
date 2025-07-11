package echo

import (
	"fmt"

	echo "github.com/labstack/echo/v4"
	"github.com/storacha/go-ucanto/principal"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/build"
)

// RootHandler provides the root route handler
type RootHandler struct {
	id principal.Signer
}

// NewRootHandler creates a new root handler
func NewRootHandler(id principal.Signer) *RootHandler {
	return &RootHandler{id: id}
}

// RegisterRoutes registers the root route
func (h *RootHandler) RegisterRoutes(e *echo.Echo) {
	e.GET("/", h.handleRoot)
}

// handleRoot displays version info
func (h *RootHandler) handleRoot(c echo.Context) error {
	response := fmt.Sprintf("🔥 piri %s\n", build.Version)
	response += "- https://github.com/storacha/piri\n"
	response += fmt.Sprintf("- %s", h.id.DID())
	
	return c.String(200, response)
}

// RootHandlerModule provides the root handler with route registrar tag
var RootHandlerModule = fx.Module("root-handler",
	fx.Provide(
		fx.Annotate(
			NewRootHandler,
			fx.ResultTags(`group:"route_registrar"`),
		),
	),
)