package echo

import (
	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
)

// ErrorHandler is a middleware that logs errors to the provided logger.
func ErrorHandler(log logging.EventLogger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)
			if err != nil {
				// do not log HTTP errors, since they have been "handled" already
				if _, ok := err.(*echo.HTTPError); !ok {
					log.Error(err)
				}
			}
			return err
		}
	}
}
