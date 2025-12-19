package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/storacha/piri/lib/telemetry/metrics"
	"github.com/storacha/piri/lib/telemetry/traces"
)

type shutdownFn func(context.Context) error

type Telemetry struct {
	Metrics     metric.MeterProvider
	Traces      trace.TracerProvider
	shutdownFns []shutdownFn
}

func New(
	ctx context.Context,
	environment, serviceName, serviceVersion, instanceID string,
	metricCollectors metrics.Config,
	tracesCollectors traces.Config,
	resourceOpts ...resource.Option,
) (*Telemetry, error) {
	if serviceName == "" {
		return nil, fmt.Errorf("telemetry service name required")
	}
	if serviceVersion == "" {
		return nil, fmt.Errorf("telemetry service version required")
	}
	if instanceID == "" {
		return nil, fmt.Errorf("telemetry instance id required")
	}
	if environment == "" {
		return nil, fmt.Errorf("telemetry environment required")
	}

	var rsrcOpts []resource.Option
	rsrcOpts = append(rsrcOpts, resource.WithAttributes(
		semconv.ServiceNameKey.String(serviceName),
		semconv.ServiceVersionKey.String(serviceVersion),
		semconv.ServiceInstanceIDKey.String(instanceID),
		semconv.DeploymentEnvironmentNameKey.String(environment),
	))
	rsrcOpts = append(rsrcOpts, resourceOpts...)

	rsrc, err := resource.New(ctx, rsrcOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	metricsProvider, metricShutdownFn, err := metrics.NewProvider(ctx, rsrc, metricCollectors)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics provider: %w", err)
	}

	traceProvider, traceShutdownFn, err := traces.NewProvider(ctx, rsrc, tracesCollectors)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace provider: %w", err)
	}

	otel.SetMeterProvider(metricsProvider)
	otel.SetTracerProvider(traceProvider)

	return &Telemetry{
			Metrics:     metricsProvider,
			Traces:      traceProvider,
			shutdownFns: []shutdownFn{metricShutdownFn, traceShutdownFn},
		},
		nil
}

func (t *Telemetry) Shutdown(ctx context.Context) error {
	for _, fn := range t.shutdownFns {
		if err := fn(ctx); err != nil {
			return err
		}
	}
	return nil
}
