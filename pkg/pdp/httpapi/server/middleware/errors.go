package middleware

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/pdp/types"
)

// CustomHTTPErrorHandler is a centralized error handler for all Echo routes
// Set this as Echo's HTTPErrorHandler to automatically handle all errors
func CustomHTTPErrorHandler(err error, c echo.Context) {
	// Don't handle if response already started
	if c.Response().Committed {
		return
	}

	HandleError(err, c)
}

// ErrorResponse represents a structured error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// typeErrorStatusMap maps types.Error kinds to HTTP status codes
var typeErrorStatusMap = map[types.Kind]int{
	types.KindNotFound:     http.StatusNotFound,
	types.KindInvalidInput: http.StatusBadRequest,
	types.KindUnauthorized: http.StatusUnauthorized,
	types.KindConflict:     http.StatusConflict,
}

// HandleError converts any error to an HTTP response
// It's especially helpful for handling our custom ContextualError
func HandleError(err error, c echo.Context) {
	if err == nil {
		return
	}

	code, message := extractErrorInfo(err)
	sendErrorResponse(c, code, message)
}

// extractErrorInfo determines the appropriate HTTP status code and message from an error
func extractErrorInfo(err error) (int, string) {
	// Handle Echo's HTTPError
	var he *echo.HTTPError
	if errors.As(err, &he) {
		return he.Code, fmt.Sprintf("%s", he.Message)
	}

	// Handle types.Error with mapped status codes
	var tErr *types.Error
	if errors.As(err, &tErr) {
		if status, ok := typeErrorStatusMap[tErr.Kind()]; ok {
			return status, tErr.Error()
		}
		return http.StatusInternalServerError, tErr.Error()
	}

	// Default case: Internal Server Error
	return http.StatusInternalServerError, err.Error()
}

// sendErrorResponse sends a JSON error response to the client
func sendErrorResponse(c echo.Context, code int, message string) {
	if err := c.JSON(code, ErrorResponse{Error: message}); err != nil {
		c.Logger().Errorf("Failed to send error response: %v", err)
	}
}
