package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Counter struct {
	counter metric.Int64Counter
	attrs   []attribute.KeyValue
}

type CounterConfig struct {
	Name        string
	Description string
	Unit        string
	Attributes  map[string]string
}

func NewCounter(meter metric.Meter, cfg CounterConfig) (*Counter, error) {
	opts := []metric.Int64CounterOption{
		metric.WithDescription(cfg.Description),
	}

	if cfg.Unit != "" {
		opts = append(opts, metric.WithUnit(cfg.Unit))
	}

	counter, err := meter.Int64Counter(cfg.Name, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create counter %s: %w", cfg.Name, err)
	}

	attrs := make([]attribute.KeyValue, 0, len(cfg.Attributes))
	for k, v := range cfg.Attributes {
		attrs = append(attrs, attribute.String(k, v))
	}

	return &Counter{
		counter: counter,
		attrs:   attrs,
	}, nil
}

func (c *Counter) Add(ctx context.Context, value int64, attrs ...attribute.KeyValue) {
	allAttrs := append(c.attrs, attrs...)
	c.counter.Add(ctx, value, metric.WithAttributes(allAttrs...))
}

func (c *Counter) Inc(ctx context.Context, attrs ...attribute.KeyValue) {
	c.Add(ctx, 1, attrs...)
}

func (c *Counter) WithAttributes(attrs ...attribute.KeyValue) *Counter {
	return &Counter{
		counter: c.counter,
		attrs:   append(c.attrs, attrs...),
	}
}

type FloatCounter struct {
	counter metric.Float64Counter
	attrs   []attribute.KeyValue
}

type FloatCounterConfig struct {
	Name        string
	Description string
	Unit        string
	Attributes  map[string]string
}

func NewFloatCounter(meter metric.Meter, cfg FloatCounterConfig) (*FloatCounter, error) {
	opts := []metric.Float64CounterOption{
		metric.WithDescription(cfg.Description),
	}

	if cfg.Unit != "" {
		opts = append(opts, metric.WithUnit(cfg.Unit))
	}

	counter, err := meter.Float64Counter(cfg.Name, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create float counter %s: %w", cfg.Name, err)
	}

	attrs := make([]attribute.KeyValue, 0, len(cfg.Attributes))
	for k, v := range cfg.Attributes {
		attrs = append(attrs, attribute.String(k, v))
	}

	return &FloatCounter{
		counter: counter,
		attrs:   attrs,
	}, nil
}

func (c *FloatCounter) Add(ctx context.Context, value float64, attrs ...attribute.KeyValue) {
	allAttrs := append(c.attrs, attrs...)
	c.counter.Add(ctx, value, metric.WithAttributes(allAttrs...))
}

func (c *FloatCounter) WithAttributes(attrs ...attribute.KeyValue) *FloatCounter {
	return &FloatCounter{
		counter: c.counter,
		attrs:   append(c.attrs, attrs...),
	}
}
