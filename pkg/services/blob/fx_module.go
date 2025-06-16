package blob

import (
	"context"

	"github.com/labstack/echo/v4"
	ucanserver "github.com/storacha/go-ucanto/server"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/services/blob/http"
	"github.com/storacha/piri/pkg/services/blob/ucan"
	"github.com/storacha/piri/pkg/services/types"
)

// ServiceModule provides only the blob service implementation.
// For HTTP server functionality, compose with blob/http.Module
var ServiceModule = fx.Module("blob-service",
	// Provide the service implementation
	// The concrete *Service type also implements the types.Blobs interface
	fx.Provide(
		fx.Annotate(
			NewService,
			fx.As(new(types.Blobs)),
		),
	),
)

var HTTPModule = fx.Module("blob-http",
	// Provide the server
	fx.Provide(http.NewBlob),
	// Register routes on startup
	fx.Invoke(registerRoutes),
)

// registerRoutes registers the blob server routes during app startup
func registerRoutes(lc fx.Lifecycle, e *echo.Echo, handler *http.Blob) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			handler.RegisterRoutes(e)
			return nil
		},
	})
}

var UCANModule = fx.Module("blob-ucan",
	fx.Provide(
		ucan.NewAllocate,
		ucan.NewAccept,
	),
	// Provide server options with tags
	fx.Provide(
		fx.Annotate(
			func(h *ucan.AllocateMethod) ucanserver.Option { return h.Option() },
			fx.ResultTags(`group:"ucan-options"`),
		),
		fx.Annotate(
			func(h *ucan.AcceptMethod) ucanserver.Option { return h.Option() },
			fx.ResultTags(`group:"ucan-options"`),
		),
	),

)
