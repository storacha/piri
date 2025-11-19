package telemetry

import (
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
)

type otlpHTTPOptions struct {
	endpoint string
	insecure bool
	headers  map[string]string
}

func newOTLPHTTPOptions(endpoint string, insecure bool, headers map[string]string) otlpHTTPOptions {
	return otlpHTTPOptions{
		endpoint: endpoint,
		insecure: insecure,
		headers:  headers,
	}
}

func (o otlpHTTPOptions) metricOptions() []otlpmetrichttp.Option {
	return buildHTTPOptions(o, otlpmetrichttp.WithEndpoint, otlpmetrichttp.WithInsecure, otlpmetrichttp.WithHeaders)
}

func (o otlpHTTPOptions) traceOptions() []otlptracehttp.Option {
	return buildHTTPOptions(o, otlptracehttp.WithEndpoint, otlptracehttp.WithInsecure, otlptracehttp.WithHeaders)
}

func buildHTTPOptions[T any](
	opts otlpHTTPOptions,
	withEndpoint func(string) T,
	withInsecure func() T,
	withHeaders func(map[string]string) T,
) []T {
	if opts.endpoint == "" {
		return nil
	}

	options := []T{withEndpoint(opts.endpoint)}

	if opts.insecure {
		options = append(options, withInsecure())
	}

	if len(opts.headers) > 0 {
		options = append(options, withHeaders(opts.headers))
	}

	return options
}
