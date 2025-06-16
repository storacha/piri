package http

import (
	"context"
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
	"go.uber.org/fx"

	"github.com/storacha/go-libstoracha/ipnipublisher/server"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
)

var log = logging.Logger("publisher-server")

// Publisher provides HTTP endpoints for publisher service
type Publisher struct {
	ipniServer *server.Server
}

// Params defines the dependencies for the server
type Params struct {
	fx.In
	PublisherStore store.PublisherStore
}

// NewPublisher creates a new publisher server
func NewPublisher(params Params) (*Publisher, error) {
	// Create IPNI server that serves advertisements
	encodableStore, ok := params.PublisherStore.(store.EncodeableStore)
	if !ok {
		return nil, fmt.Errorf("publisher store does not implement EncodableStore")
	}

	ipniServer, err := server.NewServer(encodableStore)
	if err != nil {
		return nil, err
	}

	return &Publisher{
		ipniServer: ipniServer,
	}, nil
}

// RegisterRoutes registers all routes for the publisher server
func (s *Publisher) RegisterRoutes(e *echo.Echo) {
	// Wrap the IPNI server handler
	ipniHandler := echo.WrapHandler(s.ipniServer)

	// Register IPNI routes
	g := e.Group(server.IPNIPath)
	g.GET("/:ad", ipniHandler)
}

func (s *Publisher) Start(ctx context.Context) error {
	return s.ipniServer.Start(ctx)
}

// Stop gracefully shuts down the IPNI server
func (s *Publisher) Stop(ctx context.Context) error {
	return s.ipniServer.Shutdown(ctx)
}
