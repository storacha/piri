package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

type Provider struct {
	provider *sdkmetric.MeterProvider
	meter    metric.Meter
}

type Config struct {
	ServiceName    string
	ServiceVersion string
	environment    string
	endpoint       string
	insecure       bool
	headers        map[string]string
}

func NewProvider(ctx context.Context, cfg Config) (*Provider, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.version", cfg.ServiceVersion),
			attribute.String("deployment.environment", cfg.environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	opts := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(cfg.endpoint),
	}

	if cfg.insecure {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}

	if len(cfg.headers) > 0 {
		opts = append(opts, otlpmetrichttp.WithHeaders(cfg.headers))
	}

	exporter, err := otlpmetrichttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(exporter,
				sdkmetric.WithInterval(30*time.Second),
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
