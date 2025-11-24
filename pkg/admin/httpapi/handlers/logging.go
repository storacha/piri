package handlers

import (
	"net/http"

	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/admin/httpapi"
)

// listLogLevels lists each logging systems and its associated level.
func listLogLevels(ctx echo.Context) error {
	systems := logging.GetSubsystems()
	loggers := make(map[string]string, len(systems))
	for _, system := range systems {
		loggers[system] = logging.Logger(system).Level().String()
	}
	return ctx.JSON(http.StatusOK, &httpapi.ListLogLevelsResponse{Loggers: loggers})
}

// setLogLevel sets the logging level of the specified system
func setLogLevel(ctx echo.Context) error {
	var req httpapi.SetLogLevelRequest
	if err := ctx.Bind(&req); err != nil {
		return err
	}
	if req.System == "" {
		return ctx.String(http.StatusBadRequest, "subsystem is required")
	}
	if req.Level == "" {
		return ctx.String(http.StatusBadRequest, "level is required")
	}
	if err := logging.SetLogLevel(req.System, req.Level); err != nil {
		return ctx.String(http.StatusBadRequest, err.Error())
	}
	return ctx.NoContent(http.StatusOK)
}

// setLogLevelRegex sets all loggers to specified level that match the expression.
func setLogLevelRegex(ctx echo.Context) error {
	var req httpapi.SetLogLevelRegexRequest
	if err := ctx.Bind(&req); err != nil {
		return err
	}
	if req.Expression == "" {
		return ctx.String(http.StatusBadRequest, "expression is required")
	}
	if req.Level == "" {
		return ctx.String(http.StatusBadRequest, "level is required")
	}
	if err := logging.SetLogLevelRegex(req.Expression, req.Level); err != nil {
		return ctx.String(http.StatusBadRequest, err.Error())
	}
	return ctx.NoContent(http.StatusOK)
}
