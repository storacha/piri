package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/fx"

	"github.com/storacha/piri/delegator/internal/config"
	"github.com/storacha/piri/delegator/internal/handlers"
)

type Server struct {
	echo     *echo.Echo
	config   *config.Config
	handlers *handlers.Handlers
}

func NewServer(cfg *config.Config, h *handlers.Handlers) *Server {
	e := echo.New()
	e.HideBanner = true

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())

	return &Server{
		echo:     e,
		config:   cfg,
		handlers: h,
	}
}

func (s *Server) setupRoutes() {
	s.echo.GET("/health", s.handlers.HealthCheck)
	s.echo.GET("/", s.handlers.Root)
	s.echo.PUT("/register", s.handlers.Register)
	s.echo.GET("/request-proof", s.handlers.RequestProof)
	s.echo.GET("/is-registered", s.handlers.IsRegistered)
}

func Start(lc fx.Lifecycle, s *Server) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			s.setupRoutes()

			go func() {
				addr := s.config.Server.Address()
				fmt.Printf("Starting server on %s\n", addr)
				if err := s.echo.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
					fmt.Printf("Server error: %v\n", err)
				}
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			fmt.Println("Shutting down server...")
			return s.echo.Shutdown(ctx)
		},
	})
}
