package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Info represents an info metric - a gauge that always has value 1.0
// Info metrics are used to expose textual information as labels.
type Info struct {
	gauge *FloatGauge
	attrs []attribute.KeyValue
}

// InfoConfig configures an info metric
type InfoConfig struct {
	Name        string
	Description string
	Labels      map[string]string
}

// NewInfo creates a new info metric that always reports 1.0
// This is useful for exposing version info, addresses, and other metadata
func NewInfo(meter metric.Meter, cfg InfoConfig) (*Info, error) {
	gauge, err := NewFloatGauge(meter, FloatGaugeConfig{
		Name:        cfg.Name,
		Description: cfg.Description,
		Unit:        "1", // Info metrics are dimensionless
	})
	if err != nil {
		return nil, err
	}

	attrs := make([]attribute.KeyValue, 0, len(cfg.Labels))
	for k, v := range cfg.Labels {
		attrs = append(attrs, attribute.String(k, v))
	}

	return &Info{
		gauge: gauge,
		attrs: attrs,
	}, nil
}

// Record records the info metric with value 1.0
func (i *Info) Record(ctx context.Context) {
	i.gauge.Record(ctx, 1.0, i.attrs...)
}

// Update updates the info metric with new label values
func (i *Info) Update(ctx context.Context, labels map[string]string) {
	attrs := make([]attribute.KeyValue, 0, len(labels))
	for k, v := range labels {
		attrs = append(attrs, attribute.String(k, v))
	}
	i.attrs = attrs
	i.Record(ctx)
}
