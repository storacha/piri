package middleware

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/storacha/piri/pkg/telemetry"
)

func HTTPMetricsMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			startTime := time.Now()
			err := next(c)
			duration := time.Since(startTime)

			// handle a situation where an error occurs before a response is written
			statusCode := c.Response().Status
			if err != nil && statusCode == 0 {
				statusCode = 500
			}

			ctx := c.Request().Context()
			telemetry.RecordHTTPRequest(
				ctx,
				c.Request().Method,
				c.Path(),
				c.Request().URL.Path,
				statusCode,
				duration,
				c.Request().ContentLength,
				c.Response().Size,
			)

			return err
		}
	}
}
