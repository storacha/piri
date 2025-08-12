package echo

import (
	"context"
	"fmt"
	"net/http"

	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	pirimiddleware "github.com/storacha/piri/pkg/pdp/httpapi/server/middleware"
)

var log = logging.Logger("fx/echo")

var Module = fx.Module("echo",
	fx.Provide(
		NewEcho,
	),
	fx.Invoke(
		RegisterRoutes,
		StartEchoServer,
	),
)

// RouteRegistrar defines the interface for services that register Echo routes
type RouteRegistrar interface {
	RegisterRoutes(e *echo.Echo)
}

// NewEcho creates a new Echo instance with default middleware
func NewEcho() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Add default middleware
	e.Use(pirimiddleware.LogMiddleware(logging.Logger("server")))
	e.Use(middleware.Recover())

	return e
}

// EchoServer wraps Echo with fx lifecycle management
type EchoServer struct {
	echo *echo.Echo
	addr string
}

// StartEchoServer runs a Echo server with lifecycle management
func StartEchoServer(cfg app.AppConfig, e *echo.Echo, lc fx.Lifecycle) (*EchoServer, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	server := &EchoServer{
		echo: e,
		addr: addr,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Infof("Starting Echo server on %s", addr)

			// Start server in a goroutine
			go func() {
				if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
					log.Errorf("Echo server error: %v", err)
				}
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info("Shutting down Echo server")
			return e.Shutdown(ctx)
		},
	})

	return server, nil
}

// RouteParams collects all route registrars
type RouteParams struct {
	fx.In

	Registrars []RouteRegistrar `group:"route_registrar"`
}

// RegisterRoutes registers all routes from collected registrars
func RegisterRoutes(e *echo.Echo, params RouteParams) {
	log.Infof("Registering routes from %d registrars", len(params.Registrars))

	for _, registrar := range params.Registrars {
		registrar.RegisterRoutes(e)
	}
}

// Address returns the server's listening address
func (s *EchoServer) Address() string {
	return s.addr
}
