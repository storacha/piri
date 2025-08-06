
package admin

import (
	"net/http"

	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
)

// RegisterAdminRoutes registers the admin API routes on the given echo server.
func RegisterAdminRoutes(e *echo.Echo) {
	e.GET("/log/level", listLogLevels)
	e.POST("/log/level", setLogLevel)
}

// ListLogLevelsResponse defines the response for the list log levels endpoint.
type ListLogLevelsResponse struct {
	Levels map[string]string `json:"levels"`
}

// SetLogLevelRequest defines the request for the set log level endpoint.
type SetLogLevelRequest struct {
	Subsystem string `json:"subsystem"`
	Level     string `json:"level"`
}

func listLogLevels(c echo.Context) error {
	levels := make(map[string]string)
	for _, subsystem := range logging.GetSubsystems() {
		levels[subsystem] = logging.Logger(subsystem).Level().String()
	}
	return c.JSON(http.StatusOK, &ListLogLevelsResponse{Levels: levels})
}

func setLogLevel(c echo.Context) error {
	var req SetLogLevelRequest
	if err := c.Bind(&req); err != nil {
		return err
	}

	if req.Subsystem == "" {
		return c.String(http.StatusBadRequest, "subsystem is required")
	}
	if req.Level == "" {
		return c.String(http.StatusBadRequest, "level is required")
	}

	if err := logging.SetLogLevel(req.Subsystem, req.Level); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	return c.NoContent(http.StatusOK)
}
