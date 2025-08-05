package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Info represents an info metric - a gauge that always has value 1
// Info metrics are used to expose textual information as labels.
type Info struct {
	gauge *Gauge
	attrs map[string]attribute.KeyValue
}

// InfoConfig configures an info metric
type InfoConfig struct {
	Name        string
	Description string
	Labels      map[string]string
}

// NewInfo creates a new info metric. Under the hood it is a gauge that always reports 1.
// This is useful for exposing version info, addresses, and other metadata
func NewInfo(meter metric.Meter, cfg InfoConfig) (*Info, error) {
	gauge, err := NewGauge(meter, GaugeConfig{
		Name:        cfg.Name,
		Description: cfg.Description,
	})
	if err != nil {
		return nil, err
	}

	attrs := make(map[string]attribute.KeyValue, len(cfg.Labels))
	for k, v := range cfg.Labels {
		attrs[k] = attribute.String(k, v)
	}

	return &Info{
		gauge: gauge,
		attrs: attrs,
	}, nil
}

// Record records the info metric with the given attributes, merging them with existing ones
func (i *Info) Record(ctx context.Context, attrs ...attribute.KeyValue) {
	// Update the stored attributes
	for _, attr := range attrs {
		i.attrs[string(attr.Key)] = attr
	}

	recordedAttrs := make([]attribute.KeyValue, 0, len(i.attrs))
	for _, v := range i.attrs {
		recordedAttrs = append(recordedAttrs, v)
	}

	i.gauge.Record(ctx, 1, recordedAttrs...)
}
