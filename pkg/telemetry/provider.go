package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
)

type Provider struct {
	provider *sdkmetric.MeterProvider
	meter    metric.Meter
}

type Config struct {
	ServiceName     string
	ServiceVersion  string
	PublishInterval time.Duration
	TracesEndpoint  string
	environment     string
	endpoint        string
	insecure        bool
	headers         map[string]string
}

func newProvider(ctx context.Context, cfg Config, res *sdkresource.Resource, opts []otlpmetrichttp.Option) (*Provider, error) {
	if len(opts) == 0 {
		return nil, fmt.Errorf("metrics endpoint is required")
	}

	exporter, err := otlpmetrichttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(exporter,
				sdkmetric.WithInterval(cfg.PublishInterval),
			),
		),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(provider)

	return &Provider{
		provider: provider,
		meter:    provider.Meter(cfg.ServiceName),
	}, nil
}

func (p *Provider) Meter() metric.Meter {
	return p.meter
}

func (p *Provider) Shutdown(ctx context.Context) error {
	return p.provider.Shutdown(ctx)
}
