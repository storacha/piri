package telemetry

import (
	"context"
	"log"
	"net/http"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/labstack/echo/v4"
	"github.com/storacha/piri/pkg/build"
)

// HTTPError is an error that also has an associated HTTP status code
type HTTPError struct {
	err        error
	statusCode int
}

// Error implements the error interface
func (he HTTPError) Error() string {
	return he.err.Error()
}

// StatusCode returns the HTTP status code associated with the error
func (he HTTPError) StatusCode() int {
	return he.statusCode
}

// NewHTTPError creates a new HTTPError
func NewHTTPError(err error, statusCode int) HTTPError {
	return HTTPError{err: err, statusCode: statusCode}
}

// ErrorReturningHTTPHandler is a HTTP handler function that returns an error
type ErrorReturningHTTPHandler func(http.ResponseWriter, *http.Request) error

// SetupErrorReporting configures the Sentry SDK for error reporting
func SetupErrorReporting(sentryDSN, environment string) {
	err := sentry.Init(sentry.ClientOptions{
		Dsn:           sentryDSN,
		Environment:   environment,
		Release:       build.Version,
		Transport:     sentry.NewHTTPSyncTransport(),
		EnableTracing: false,
	})

	if err != nil {
		log.Fatalf("sentry.Init: %s", err)
	}
}

// NewErrorReportingHandler wraps an ErrorReturningHTTPHandler with error reporting
func NewErrorReportingHandler(errorReturningHandler ErrorReturningHTTPHandler) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := errorReturningHandler(w, r); err != nil {
			ReportError(r.Context(), err)

			if httperr, ok := err.(*echo.HTTPError); ok {
				http.Error(w, http.StatusText(httperr.Code), httperr.Code)
				// if the error is an HTTPError, send an appropriate response aside from reporting it
			} else if e, ok := err.(HTTPError); ok {
				http.Error(w, e.Error(), e.StatusCode())
			}
		}
	})

	sentryHandler := sentryhttp.New(sentryhttp.Options{})
	return sentryHandler.Handle(handler)
}

// ReportError reports an error to Sentry
func ReportError(ctx context.Context, err error) {
	hub := sentry.GetHubFromContext(ctx)
	if hub != nil {
		hub.CaptureException(err)
	} else {
		sentry.CaptureException(err)
	}
}
