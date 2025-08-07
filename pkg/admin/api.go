package admin

import (
	"net/http"

	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
)

// RegisterAdminRoutes registers the admin API routes on the given echo server.
func RegisterAdminRoutes(e *echo.Echo) {
	e.GET("/log/subsystems", listLogSubsystems)
	e.GET("/log/level", listLogLevels)
	e.POST("/log/level", setLogLevel)
}

// ListLogSubsystemsResponse defines the response for the list log subsystems endpoint.
type ListLogSubsystemsResponse struct {
	Subsystems []string `json:"subsystems"`
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

func listLogSubsystems(c echo.Context) error {
	return c.JSON(http.StatusOK, &ListLogSubsystemsResponse{
		Subsystems: logging.GetSubsystems(),
	})
}

func listLogLevels(c echo.Context) error {
	// First get the list of subsystems from the server
	subsystems := logging.GetSubsystems()
	levels := make(map[string]string, len(subsystems))

	// Then get the level for each subsystem
	for _, subsystem := range subsystems {
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
