package echo

import (
	"errors"

	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
)

// ErrorLogger is a middleware that logs errors to the provided logger.
func ErrorLogger(log logging.EventLogger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)
			if err != nil {
				// do not log HTTP errors, since they have been "handled" already
				var HTTPError *echo.HTTPError
				if !errors.As(err, &HTTPError) {
					log.Error(err)
				}
			}
			return err
		}
	}
}
