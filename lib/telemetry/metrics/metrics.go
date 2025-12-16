package metrics

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

type CollectorConfig struct {
	Endpoint        string
	Insecure        bool
	Headers         map[string]string
	PublishInterval time.Duration
}

type Config struct {
	Collectors []CollectorConfig
	Options    []sdkmetric.Option
}

func NewProvider(
	ctx context.Context,
	res *resource.Resource,
	cfg Config,
) (metric.MeterProvider, func(ctx2 context.Context) error, error) {
	if len(cfg.Collectors) == 0 {
		return noop.NewMeterProvider(),
			func(ctx context.Context) error { return nil },
			nil
	}

	var readers []sdkmetric.Reader
	for _, collector := range cfg.Collectors {
		if collector.Endpoint == "" {
			return nil, nil, fmt.Errorf("telemetry provider endpoint is required")
		}
		if collector.PublishInterval == 0 {
			return nil, nil, fmt.Errorf("telemetry provider publish interval is required")
		}
		opts := []otlpmetrichttp.Option{
			otlpmetrichttp.WithEndpoint(collector.Endpoint),
			otlpmetrichttp.WithHeaders(collector.Headers),
		}
		if collector.Insecure {
			opts = append(opts, otlpmetrichttp.WithInsecure())
		}
		exporter, err := otlpmetrichttp.New(ctx, opts...)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create metrics provider: %w", err)
		}
		readers = append(readers, sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(collector.PublishInterval)))
	}

	// attach the resource
	providerOptions := []sdkmetric.Option{
		sdkmetric.WithResource(res),
	}
	// attach reader(s)
	for _, r := range readers {
		providerOptions = append(providerOptions, sdkmetric.WithReader(r))
	}
	// any remaining options
	providerOptions = append(providerOptions, cfg.Options...)

	provider := sdkmetric.NewMeterProvider(providerOptions...)
	return provider, provider.Shutdown, nil
}
