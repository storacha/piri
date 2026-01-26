package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/admin/httpapi"
	"github.com/storacha/piri/pkg/config/dynamic"
)

// ConfigHandler handles dynamic configuration API requests.
type ConfigHandler struct {
	registry *dynamic.Registry
	bridge   *dynamic.ViperBridge
}

// NewConfigHandler creates a new ConfigHandler.
func NewConfigHandler(registry *dynamic.Registry, bridge *dynamic.ViperBridge) *ConfigHandler {
	return &ConfigHandler{
		registry: registry,
		bridge:   bridge,
	}
}

// GetConfig returns the current dynamic configuration values.
// GET /admin/config
func (h *ConfigHandler) GetConfig(c echo.Context) error {
	return c.JSON(http.StatusOK, httpapi.ConfigResponse{
		Values: h.registry.GetAll(),
	})
}

// UpdateConfig updates one or more dynamic configuration values.
// PATCH /admin/config
func (h *ConfigHandler) UpdateConfig(c echo.Context) error {
	var req httpapi.UpdateConfigRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest,
			fmt.Sprintf("invalid request body: %s", err))
	}

	if len(req.Updates) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "no updates provided")
	}

	// Registry handles ALL validation (parsing, type checking, constraints)
	if err := h.registry.Update(req.Updates, req.Persist, dynamic.SourceAPI); err != nil {
		return h.mapError(err)
	}

	return c.JSON(http.StatusOK, httpapi.ConfigResponse{
		Values: h.registry.GetAll(),
	})
}

// ReloadConfig reloads the configuration from the config file.
// POST /admin/config/reload
func (h *ConfigHandler) ReloadConfig(c echo.Context) error {
	if h.bridge == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "config reload not available (no config file)")
	}

	if err := h.bridge.Reload(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError,
			fmt.Sprintf("failed to reload config: %s", err))
	}

	return c.JSON(http.StatusOK, httpapi.ConfigResponse{
		Values: h.registry.GetAll(),
	})
}

// mapError maps dynamic config errors to appropriate HTTP errors.
func (h *ConfigHandler) mapError(err error) *echo.HTTPError {
	var validationErr *dynamic.ValidationError
	var unknownKeyErr *dynamic.UnknownKeyError
	var persistErr *dynamic.PersistError

	switch {
	case errors.As(err, &validationErr):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	case errors.As(err, &unknownKeyErr):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	case errors.As(err, &persistErr):
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
}
