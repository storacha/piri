package claim

import (
	"context"

	"github.com/labstack/echo/v4"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/services/claim/http"
	"github.com/storacha/piri/pkg/services/types"
)

// ServiceModule provides only the claim service implementation.
// For HTTP server functionality, compose with claim/http.ServiceModule
var ServiceModule = fx.Module("claim-service",
	// Provide the service implementation
	// The concrete *Service type also implements the types.Claims interface
	fx.Provide(
		fx.Annotate(
			NewService,
			fx.As(new(types.Claims)),
		),
	),
)

var HTTPModule = fx.Module("claim-http",
	// Provide the server
	fx.Provide(http.NewClaimHandler),
	// Register routes on startup
	fx.Invoke(registerRoutes),
)

// registerRoutes registers the claim server routes during app startup
func registerRoutes(lc fx.Lifecycle, e *echo.Echo, handler *http.Claim) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			handler.RegisterRoutes(e)
			return nil
		},
	})
}
