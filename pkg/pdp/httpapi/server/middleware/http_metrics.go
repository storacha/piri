package middleware

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/storacha/piri/pkg/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func HTTPMetricsMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			startTime := time.Now()
			err := next(c)
			duration := time.Since(startTime).Seconds()

			// handle a situation where an error occurs before a response is written
			statusCode := c.Response().Status
			if err != nil && statusCode == 0 {
				statusCode = 500
			}

			opts := metric.WithAttributes(
				attribute.String("http.method", c.Request().Method),
				attribute.String("http.route", c.Path()),
				attribute.String("url.path", c.Request().URL.Path),
				attribute.Int("http.status_code", statusCode),
			)

			ctx := c.Request().Context()

			// only record request size if it's known
			if reqSize := c.Request().ContentLength; reqSize > 0 {
				telemetry.HTTPRequestSize.Record(ctx, float64(reqSize), opts)
			}
			telemetry.HTTPRequestDuration.Record(ctx, duration, opts)
			telemetry.HTTPRequestsTotal.Add(ctx, 1, opts)
			telemetry.HTTPResponseSize.Record(ctx, float64(c.Response().Size), opts)

			return err
		}
	}
}
