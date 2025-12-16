package traces

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type Config struct {
	Collectors []CollectorConfig
	Options    []sdktrace.TracerProviderOption
}

type CollectorConfig struct {
	Endpoint        string
	Insecure        bool
	Headers         map[string]string
	PublishInterval time.Duration
}

func NewProvider(
	ctx context.Context,
	res *sdkresource.Resource,
	cfg Config,
) (trace.TracerProvider, func(ctx context.Context) error, error) {
	defaultPropagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	if len(cfg.Collectors) == 0 {
		return noop.NewTracerProvider(),
			func(ctx context.Context) error { return nil },
			nil
	}

	var processors []sdktrace.SpanProcessor
	for _, collector := range cfg.Collectors {
		if collector.Endpoint == "" {
			return nil, nil, fmt.Errorf("collector endpoint required")
		}
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(collector.Endpoint),
			otlptracehttp.WithHeaders(collector.Headers),
		}
		if collector.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		exporter, err := otlptracehttp.New(ctx, opts...)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create trace exporter: %w", err)
		}
		var bspOpts []sdktrace.BatchSpanProcessorOption
		if collector.PublishInterval > 0 {
			bspOpts = append(bspOpts, sdktrace.WithBatchTimeout(collector.PublishInterval))
		}
		processors = append(processors, sdktrace.NewBatchSpanProcessor(exporter, bspOpts...))
	}

	// attach resource
	providerOptions := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
	}
	// all processors (endpoints namely)
	for _, p := range processors {
		providerOptions = append(providerOptions, sdktrace.WithSpanProcessor(p))
	}
	// remaining options
	providerOptions = append(providerOptions, cfg.Options...)

	provider := sdktrace.NewTracerProvider(providerOptions...)
	otel.SetTextMapPropagator(defaultPropagator)
	return provider, provider.Shutdown, nil
}
