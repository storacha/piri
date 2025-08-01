package apiv2

import (
	"errors"
	"fmt"
	"net/http"
)

// APIError represents an error with an associated HTTP status code
type APIError struct {
	StatusCode int
	Message    string
	Err        error // Underlying error
}

func (e *APIError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *APIError) Unwrap() error {
	return e.Err
}

// WrapError wraps an existing error with HTTP status information
func WrapError(err error, statusCode int, message string, args ...interface{}) *APIError {
	return &APIError{
		StatusCode: statusCode,
		Message:    fmt.Sprintf(message, args...),
		Err:        err,
	}
}

func NewError(statusCode int, message string, args ...interface{}) *APIError {
	return &APIError{
		StatusCode: statusCode,
		Message:    fmt.Sprintf(message, args...),
	}
}

// GetAPIError extracts API error information from any error
func GetAPIError(err error) (int, string) {
	if err == nil {
		return http.StatusOK, ""
	}

	// Check if it's already an APIError
	var httpErr *APIError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode, httpErr.Message
	}

	// Default to internal server error
	return http.StatusInternalServerError, err.Error()
}
