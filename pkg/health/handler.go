package health

import (
	"net/http"

	"github.com/labstack/echo/v4"

	echofx "github.com/storacha/piri/pkg/fx/echo"
)

var _ echofx.RouteRegistrar = (*Handler)(nil)

// Handler provides health check HTTP handlers
type Handler struct {
	checker *Checker
}

// NewHandler creates a new health handler
func NewHandler(checker *Checker) *Handler {
	return &Handler{checker: checker}
}

// RegisterRoutes implements echofx.RouteRegistrar
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.GET("/healthz", h.Health)
	e.GET("/livez", h.Liveness)
	e.GET("/readyz", h.Readiness)
}

// Health handles the /healthz endpoint
func (h *Handler) Health(c echo.Context) error {
	resp := h.checker.HealthCheck()
	status := http.StatusOK
	if resp.Status != StatusOK {
		status = http.StatusServiceUnavailable
	}
	return c.JSON(status, resp)
}

// Liveness handles the /livez endpoint
func (h *Handler) Liveness(c echo.Context) error {
	resp := h.checker.LivenessCheck()
	return c.JSON(http.StatusOK, resp)
}

// Readiness handles the /readyz endpoint
func (h *Handler) Readiness(c echo.Context) error {
	resp := h.checker.ReadinessCheck()
	status := http.StatusOK
	if resp.Status != StatusOK {
		status = http.StatusServiceUnavailable
	}
	return c.JSON(status, resp)
}
