package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Gauge struct {
	gauge metric.Int64Gauge
	attrs []attribute.KeyValue
}

type GaugeConfig struct {
	Name        string
	Description string
	Unit        string
	Attributes  map[string]string
}

func NewGauge(meter metric.Meter, cfg GaugeConfig) (*Gauge, error) {
	opts := []metric.Int64GaugeOption{
		metric.WithDescription(cfg.Description),
	}

	if cfg.Unit != "" {
		opts = append(opts, metric.WithUnit(cfg.Unit))
	}

	gauge, err := meter.Int64Gauge(cfg.Name, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create gauge %s: %w", cfg.Name, err)
	}

	attrs := make([]attribute.KeyValue, 0, len(cfg.Attributes))
	for k, v := range cfg.Attributes {
		attrs = append(attrs, attribute.String(k, v))
	}

	return &Gauge{
		gauge: gauge,
		attrs: attrs,
	}, nil
}

func (g *Gauge) Record(ctx context.Context, value int64, attrs ...attribute.KeyValue) {
	allAttrs := append(g.attrs, attrs...)
	g.gauge.Record(ctx, value, metric.WithAttributes(allAttrs...))
}

func (g *Gauge) WithAttributes(attrs ...attribute.KeyValue) *Gauge {
	return &Gauge{
		gauge: g.gauge,
		attrs: append(g.attrs, attrs...),
	}
}

type FloatGauge struct {
	gauge metric.Float64Gauge
	attrs []attribute.KeyValue
}

type FloatGaugeConfig struct {
	Name        string
	Description string
	Unit        string
	Attributes  map[string]string
}

func NewFloatGauge(meter metric.Meter, cfg FloatGaugeConfig) (*FloatGauge, error) {
	opts := []metric.Float64GaugeOption{
		metric.WithDescription(cfg.Description),
	}

	if cfg.Unit != "" {
		opts = append(opts, metric.WithUnit(cfg.Unit))
	}

	gauge, err := meter.Float64Gauge(cfg.Name, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create float gauge %s: %w", cfg.Name, err)
	}

	attrs := make([]attribute.KeyValue, 0, len(cfg.Attributes))
	for k, v := range cfg.Attributes {
		attrs = append(attrs, attribute.String(k, v))
	}

	return &FloatGauge{
		gauge: gauge,
		attrs: attrs,
	}, nil
}

func (g *FloatGauge) Record(ctx context.Context, value float64, attrs ...attribute.KeyValue) {
	allAttrs := append(g.attrs, attrs...)
	g.gauge.Record(ctx, value, metric.WithAttributes(allAttrs...))
}

func (g *FloatGauge) WithAttributes(attrs ...attribute.KeyValue) *FloatGauge {
	return &FloatGauge{
		gauge: g.gauge,
		attrs: append(g.attrs, attrs...),
	}
}