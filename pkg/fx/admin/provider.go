package admin

import (
	"net/http"
	"sync"

	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
	"go.uber.org/fx"

	echofx "github.com/storacha/piri/pkg/fx/echo"
)

var log = logging.Logger("fx/admin")

// Module provides admin endpoints including shutdown
var Module = fx.Module("admin",
	fx.Provide(
		fx.Annotate(
			NewAdminHandler,
			fx.As(new(echofx.RouteRegistrar)),
			fx.ResultTags(`group:"route_registrar"`),
		),
	),
)

var _ echofx.RouteRegistrar = (*Handler)(nil)

// Handler provides admin endpoints
type Handler struct {
	shutdowner fx.Shutdowner
	mu         sync.Mutex
	shutting   bool
}

// NewAdminHandler creates a new admin handler with shutdown capability
func NewAdminHandler(shutdowner fx.Shutdowner) *Handler {
	return &Handler{
		shutdowner: shutdowner,
	}
}

// RegisterRoutes registers admin routes
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	admin := e.Group("/admin")
	admin.POST("/shutdown", h.handleShutdown)
}

// handleShutdown handles shutdown requests
func (h *Handler) handleShutdown(c echo.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if already shutting down
	if h.shutting {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "server is already shutting down",
		})
	}

	h.shutting = true
	log.Info("received shutdown request via admin endpoint")

	// Trigger graceful shutdown synchronously
	// With fx.Run(), fx.Shutdowner works properly
	if err := h.shutdowner.Shutdown(); err != nil {
		log.Errorf("failed to shutdown gracefully: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to initiate shutdown",
		})
	}

	return c.JSON(http.StatusAccepted, map[string]string{
		"message": "shutdown initiated",
	})
}