package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

func newResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		semconv.ServiceNameKey.String(cfg.ServiceName),              // e.g "piri"
		semconv.ServiceVersionKey.String(cfg.ServiceVersion),        // e.g. "v0.1.0"
		semconv.ServiceInstanceIDKey.String(cfg.InstanceID),         // nodes DID
		semconv.ServerAddressKey.String(cfg.Endpoint),               // e.g. https://spicystorage.tech (endpoint as advertised to network
		attribute.String("deployment.environment", cfg.Environment), // i.e. "staging", "production", etc.
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(attrs...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	return res, nil
}
