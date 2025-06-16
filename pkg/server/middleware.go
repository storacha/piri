package server

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/fx"
)

// ProvideLoggerMiddleware provides the Echo logger middleware
func ProvideLoggerMiddleware() echo.MiddlewareFunc {
	return middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format:           "[${time_rfc3339}] ${status} ${method} ${uri} ${latency_human}\n",
		CustomTimeFormat: time.RFC3339,
	})
}

// ProvideRecoverMiddleware provides the Echo recover middleware
func ProvideRecoverMiddleware() echo.MiddlewareFunc {
	return middleware.Recover()
}

// DefaultMiddlewareModule provides common middleware for the Echo server
var DefaultMiddlewareModule = fx.Module("echo-default-middleware",
	fx.Provide(
		fx.Annotate(
			ProvideLoggerMiddleware,
			fx.ResultTags(`group:"echo-middleware"`),
		),
		fx.Annotate(
			ProvideRecoverMiddleware,
			fx.ResultTags(`group:"echo-middleware"`),
		),
	),
)
