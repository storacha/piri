package management

import (
	"net/http"

	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
	"github.com/storacha/piri/pkg/admin"
)

func NewServer() *echo.Echo {
	e := echo.New()
	e.GET("/log/level", listLogLevels)
	e.POST("/log/level", setLogLevel)
	return e
}

func listLogLevels(c echo.Context) error {
	levels := make(map[string]string)
	for _, subsystem := range logging.GetSubsystems() {
		levels[subsystem] = logging.Logger(subsystem).Level().String()
	}
	return c.JSON(http.StatusOK, levels)
}

func setLogLevel(c echo.Context) error {
	var req admin.SetLogLevelRequest
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
