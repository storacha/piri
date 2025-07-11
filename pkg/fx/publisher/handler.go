package publisher

import (
	"errors"
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/storacha/go-libstoracha/ipnipublisher/server"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"

	publisherSvc "github.com/storacha/piri/pkg/service/publisher"
)

// Handler wraps publisher handler functionality for Echo
type Handler struct {
	server *server.Server
}

// NewHandler creates a new publisher handler from the publisher service
func NewHandler(pub *publisherSvc.PublisherService) (*Handler, error) {
	publisherStore := pub.Store()
	encodableStore, ok := publisherStore.(store.EncodeableStore)
	if !ok {
		return nil, errors.New("publisher store does not implement EncodableStore")
	}

	srv, err := server.NewServer(encodableStore)
	if err != nil {
		return nil, err
	}
	return &Handler{server: srv}, nil
}

// RegisterRoutes registers the publisher routes with Echo
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	// Use Echo's parameter syntax for the ad parameter
	route := fmt.Sprintf("%s/:ad", server.IPNIPath)
	e.GET(route, echo.WrapHandler(h.server))
}
