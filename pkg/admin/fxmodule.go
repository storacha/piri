package admin

import (
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/admin/httpapi/handlers"
	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/config/dynamic"
	echofx "github.com/storacha/piri/pkg/fx/echo"
)

// provideAdminRoutes wraps handlers.NewRoutes to handle optional dynamic.Registry.
func provideAdminRoutes(identity app.IdentityConfig, registry *dynamic.Registry, bridge *dynamic.ViperBridge) (echofx.RouteRegistrar, error) {
	return handlers.NewRoutes(handlers.AdminRoutesParams{
		Identity: identity,
		Registry: registry,
		Bridge:   bridge,
	})
}

var Module = fx.Module("admin",
	fx.Provide(
		fx.Annotate(
			provideAdminRoutes,
			fx.As(new(echofx.RouteRegistrar)),
			fx.ResultTags(`group:"route_registrar"`),
		),
	),
)
