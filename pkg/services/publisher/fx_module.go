package publisher

import (
	"context"

	"github.com/labstack/echo/v4"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/services/publisher/http"
	"github.com/storacha/piri/pkg/services/types"
)

// ServiceModule provides only the publisher service implementation.
// For HTTP server functionality, compose with publisher/http.ServiceModule
var ServiceModule = fx.Module("publisher-service",
	// Provide the service implementation
	fx.Provide(
		fx.Annotate(
			NewService,
			fx.As(new(types.Publisher)),
		),
	),
)

// ServiceModule provides the publisher HTTP handlers
var HTTPModule = fx.Module("publisher-http",
	// Provide the server
	fx.Provide(http.NewPublisher),

	// Register routes on startup
	fx.Invoke(registerRoutes),
)

// registerRoutes registers the publisher server routes during app startup
func registerRoutes(lc fx.Lifecycle, e *echo.Echo, server *http.Publisher) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			server.RegisterRoutes(e)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return server.Stop(ctx)
		},
	})
}
