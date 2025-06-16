package ucan

import (
	"context"

	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
)

var Module = fx.Module("ucan",
	fx.Provide(
		NewRouter,
		NewServer,
	),
	fx.Invoke(registerRoutes),
)

func registerRoutes(lc fx.Lifecycle, e *echo.Echo, r *Router) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			r.RegisterRoutes(e)
			return nil
		},
	})
}
