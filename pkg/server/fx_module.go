package server

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
)

// Module provides the Echo HTTP server instance
var Module = fx.Module("echo-server",
	fx.Provide(ProvideServer),
)

// NB(forrest): this provides an example of how middleware may be injected into the server

// ModuleDefaults provides the Echo HTTP server instance with default middleware
var ModuleDefaults = fx.Module("echo-server-with-defaults",
	Module,
	DefaultMiddlewareModule,
)

// Params allows configuration of the Echo instance
type Params struct {
	fx.In
	Middleware []echo.MiddlewareFunc `group:"echo-middleware"`
}

// ProvideServer creates and configures the Echo server instance
func ProvideServer(params Params) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Apply collected middleware
	for _, m := range params.Middleware {
		e.Use(m)
	}

	return e
}
