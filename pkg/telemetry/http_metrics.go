package telemetry

import (
	semconvhttp "go.opentelemetry.io/otel/semconv/v1.37.0/httpconv"
)

var (
	HTTPServerRequestDurationInstrument = semconvhttp.ServerRequestDuration{}.Name()
	HTTPServerRequestSizeInstrument     = semconvhttp.ServerRequestBodySize{}.Name()
	HTTPServerResponseSizeInstrument    = semconvhttp.ServerResponseBodySize{}.Name()
)

// HTTPServerDurationBounds extends the default middleware buckets (0.005–10s from
// go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho/internal/semconv)
// to capture long uploads/downloads up to 10 minutes.
var HTTPServerDurationBounds = []float64{
	0.005, 0.01, 0.025, 0.05, 0.075, 0.1,
	0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10,
	30, 60, 120, 300, 600,
}
