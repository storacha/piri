package middleware

import (
	"net/http"

	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
)

func RequestLogger(logger *logging.ZapEventLogger) echo.MiddlewareFunc {
	return echomiddleware.RequestLoggerWithConfig(echomiddleware.RequestLoggerConfig{
		LogMethod:        true,
		LogLatency:       true,
		LogRemoteIP:      true,
		LogHost:          true,
		LogURI:           true,
		LogUserAgent:     true,
		LogStatus:        true,
		LogContentLength: true,
		LogResponseSize:  true,
		LogHeaders:       []string{"X-Agent-Message"},
		LogError:         true,
		LogValuesFunc: func(c echo.Context, v echomiddleware.RequestLoggerValues) error {
			fields := []zap.Field{
				zap.Int("status", v.Status),
				zap.String("method", v.Method),
				zap.String("uri", v.URI),
				zap.String("host", v.Host),
				zap.String("remote_ip", v.RemoteIP),
				zap.Duration("latency", v.Latency),
				zap.String("user_agent", v.UserAgent),
				zap.String("content_length", v.ContentLength),
				zap.Int64("response_size", v.ResponseSize),
				zap.Reflect("headers", v.Headers),
			}
			if v.Error != nil {
				fields = append(fields, zap.Error(v.Error))
			}
			switch {
			case v.Status >= http.StatusInternalServerError:
				logger.WithOptions(zap.Fields(fields...)).Error("server error")
			case v.Status >= http.StatusBadRequest:
				logger.WithOptions(zap.Fields(fields...)).Warn("client error")
			default:
				logger.WithOptions(zap.Fields(fields...)).Info("request completed")
			}
			return nil
		},
	})
}
