package server

import (
	"context"
	"net"
	"strings"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"

	"github.com/storacha/piri/pkg/pdp/httpapi/server/middleware"
)

var logger = logging.Logger("pdp/server")

type Server struct {
	e *echo.Echo
}

func NewServer(p *PDP) *Server {
	e := echo.New()
	// don't print echo stuff when we start, our logs cover this.
	e.HideBanner = true
	e.HidePort = true

	// handle panics
	e.Use(echomiddleware.Recover())
	// log requests with our logging system
	e.Use(middleware.LogMiddleware(logger))

	// Custom error handler for our ContextualError type
	e.HTTPErrorHandler = middleware.HandleError

	p.RegisterRoutes(e)

	return &Server{e: e}
}

func (s *Server) Start(addr string) error {
	errCh := make(chan error)
	go func() {
		errCh <- s.e.Start(addr)
	}()
	// wait up to one second for the server to start
	// gripe: wish `Start` behaved like a normal start method and didn't block, Run would be a better name. shakes fist at clouds.
	return waitForServerStart(s.e, errCh, time.Second)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.e.Shutdown(ctx)
}

func waitForServerStart(e *echo.Echo, errChan <-chan error, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			var addr net.Addr
			addr = e.ListenerAddr()
			if addr != nil && strings.Contains(addr.String(), ":") {
				return nil // was started
			}
		case err := <-errChan:
			return err
		}
	}
}
