package middleware

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/pkg/pdp/types"
)

// ContextualError is a richer error interface that provides additional context
// about an error that occurred during request handling.
type ContextualError interface {
	error
	// StatusCode returns the HTTP status code that should be returned to the client
	StatusCode() int
	// LogContext returns a map of additional context for logging
	LogContext() map[string]interface{}
	// PublicMessage returns a message safe to return to the client
	PublicMessage() string
	// OriginalError returns the underlying error, if any
	OriginalError() error
}

// PDPError implements the ContextualError interface
type PDPError struct {
	Operation     string                 // The operation that failed (e.g., "ProofSetAddRoot")
	Message       string                 // Internal error message (for logs)
	ClientMessage string                 // Message safe to return to clients
	Code          int                    // HTTP status code
	Err           error                  // Original error, if any
	Context       map[string]interface{} // Additional context for logging
}

// Error satisfies the error interface
func (e *PDPError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Operation, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Operation, e.Message)
}

// StatusCode returns the HTTP status code
func (e *PDPError) StatusCode() int {
	return e.Code
}

// LogContext returns context information for logging
func (e *PDPError) LogContext() map[string]interface{} {
	ctx := make(map[string]interface{})
	for k, v := range e.Context {
		ctx[k] = v
	}
	ctx["operation"] = e.Operation
	return ctx
}

// PublicMessage returns a message safe for client consumption
func (e *PDPError) PublicMessage() string {
	if e.ClientMessage != "" {
		return e.ClientMessage
	}
	return http.StatusText(e.Code)
}

// OriginalError returns the underlying error
func (e *PDPError) OriginalError() error {
	return e.Err
}

// NewError creates a new PDPError
func NewError(operation string, message string, err error, code int) *PDPError {
	return &PDPError{
		Operation:     operation,
		Message:       message,
		ClientMessage: message, // By default, use the same message (override for sensitive errors)
		Code:          code,
		Err:           err,
		Context:       make(map[string]interface{}),
	}
}

// WithContext adds context information to the error
func (e *PDPError) WithContext(key string, value interface{}) *PDPError {
	e.Context[key] = value
	return e
}

// WithPublicMessage sets a client-safe message
func (e *PDPError) WithPublicMessage(message string) *PDPError {
	e.ClientMessage = message
	return e
}

// HandleError converts any error to an HTTP response
// It's especially helpful for handling our custom ContextualError
func HandleError(err error, c echo.Context) {
	if err == nil {
		return
	}

	// Check if it's our custom error type
	var cErr ContextualError
	if errors.As(err, &cErr) {
		// Return the appropriate status code and message
		_ = c.String(cErr.StatusCode(), cErr.PublicMessage())
		return
	}

	// Handle echo's HTTPError
	var he *echo.HTTPError
	if errors.As(err, &he) {
		_ = c.String(he.Code, fmt.Sprintf("%v", he.Message))
		return
	}

	var tErr *types.Error
	if errors.As(err, &tErr) {
		switch tErr.Kind() {
		case types.KindNotFound:
			_ = c.String(http.StatusNotFound, tErr.Error())
			return
		case types.KindInvalidInput:
			_ = c.String(http.StatusBadRequest, tErr.Error())
			return
		case types.KindUnauthorized:
			_ = c.String(http.StatusUnauthorized, tErr.Error())
			return
		default:
			_ = c.String(http.StatusInternalServerError, tErr.Error())
			return
		}
	}

	// Generic error handling
	_ = c.String(http.StatusInternalServerError, "Internal server error")
}
