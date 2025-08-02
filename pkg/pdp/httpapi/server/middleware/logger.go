package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogMiddleware returns a middleware that logs requests using the IPFS go-log logger
func LogMiddleware(logger *logging.ZapEventLogger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			res := c.Response()
			start := time.Now()

			// Execute the next handler
			err := next(c)
			if err != nil {
				c.Error(err)
			}

			// Calculate latency
			stop := time.Now()
			latency := stop.Sub(start)

			// Get request ID
			id := req.Header.Get(echo.HeaderXRequestID)
			if id == "" {
				id = res.Header().Get(echo.HeaderXRequestID)
			}

			// Normalize path
			path := req.URL.Path
			if path == "" {
				path = "/"
			}

			// Log request based on status code
			statusCode := res.Status
			logMsg := fmt.Sprintf("[%d:%s] %s %s %s ", statusCode, http.StatusText(statusCode), req.Method, path, req.URL.RawQuery)
			// Choose fields based on log level
			logFields := buildLogFields(c, req, res, id, latency, err, logger.Level())

			// Log with appropriate level
			switch {
			case statusCode >= 500:
				logger.Errorw(logMsg, logFields...)
			case statusCode >= 400:
				logger.Warnw(logMsg, logFields...)
			default: // Info for status <= 400 && >= 200
				logger.Infow(logMsg, logFields...)
			}

			return err
		}
	}
}

// buildLogFields constructs log fields appropriate for the current log level
func buildLogFields(c echo.Context, req *http.Request, res *echo.Response, id string, latency time.Duration, err error, level zapcore.Level) []interface{} {
	// Base fields - always included regardless of level
	fields := []interface{}{
		"id", id,
		"latency", latency.String(),
	}

	if level == zap.DebugLevel {
		fields = append(fields,
			"remote_ip", c.RealIP(),
			"host", req.Host,
			"referer", req.Referer(),
			"user_agent", req.UserAgent(),
			"bytes_in", req.ContentLength,
			"bytes_out", res.Size,
			"content_type", res.Header().Get("Content-Type"),
		)
	}

	// Error information
	if err != nil {
		fields = append(fields, "error", err.Error(), "error_type", getErrorType(err))

		// Enhanced logging for ContextualError
		if contextErr, ok := err.(ContextualError); ok {
			// At info level, only include operation
			if level <= zap.InfoLevel {
				if operation, ok := contextErr.LogContext()["operation"]; ok {
					fields = append(fields, "operation", operation)
				}
				if origErr := contextErr.OriginalError(); origErr != nil {
					fields = append(fields, "cause", origErr.Error())
				}
			} else {
				// For warn and error levels, include all context fields
				for k, v := range contextErr.LogContext() {
					fields = append(fields, k, v)
				}
				if origErr := contextErr.OriginalError(); origErr != nil {
					fields = append(fields, "cause", origErr.Error())
				}
			}
		}
	}

	return fields
}

// getErrorType extracts the type name of the error
func getErrorType(err error) string {
	if err == nil {
		return ""
	}

	// Get the type of the error as a string
	var HTTPError *echo.HTTPError
	switch {
	case errors.As(err, &HTTPError):
		return "echo.HTTPError"
	case isContextualError(err):
		return "ContextualError"
	default:
		return "error"
	}
}

// isContextualError checks if the error implements our ContextualError interface
func isContextualError(err error) bool {
	_, ok := err.(ContextualError)
	return ok
}
