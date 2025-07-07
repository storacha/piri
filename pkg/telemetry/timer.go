package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Timer struct {
	histogram metric.Float64Histogram
	attrs     []attribute.KeyValue
}

type TimerConfig struct {
	Name        string
	Description string
	Unit        string
	Attributes  map[string]string
	Boundaries  []float64
}

func NewTimer(meter metric.Meter, cfg TimerConfig) (*Timer, error) {
	opts := []metric.Float64HistogramOption{
		metric.WithDescription(cfg.Description),
	}

	if cfg.Unit == "" {
		cfg.Unit = "ms"
	}
	opts = append(opts, metric.WithUnit(cfg.Unit))

	if len(cfg.Boundaries) > 0 {
		opts = append(opts, metric.WithExplicitBucketBoundaries(cfg.Boundaries...))
	}

	histogram, err := meter.Float64Histogram(cfg.Name, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create timer %s: %w", cfg.Name, err)
	}

	attrs := make([]attribute.KeyValue, 0, len(cfg.Attributes))
	for k, v := range cfg.Attributes {
		attrs = append(attrs, attribute.String(k, v))
	}

	return &Timer{
		histogram: histogram,
		attrs:     attrs,
	}, nil
}

func (t *Timer) Record(ctx context.Context, duration time.Duration, attrs ...attribute.KeyValue) {
	allAttrs := append(t.attrs, attrs...)
	t.histogram.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(allAttrs...))
}

func (t *Timer) RecordFloat(ctx context.Context, value float64, attrs ...attribute.KeyValue) {
	allAttrs := append(t.attrs, attrs...)
	t.histogram.Record(ctx, value, metric.WithAttributes(allAttrs...))
}

func (t *Timer) WithAttributes(attrs ...attribute.KeyValue) *Timer {
	return &Timer{
		histogram: t.histogram,
		attrs:     append(t.attrs, attrs...),
	}
}

type TimedContext struct {
	ctx       context.Context
	timer     *Timer
	startTime time.Time
	attrs     []attribute.KeyValue
}

func (t *Timer) Start(ctx context.Context, attrs ...attribute.KeyValue) *TimedContext {
	return &TimedContext{
		ctx:       ctx,
		timer:     t,
		startTime: time.Now(),
		attrs:     attrs,
	}
}

func (tc *TimedContext) End(attrs ...attribute.KeyValue) {
	duration := time.Since(tc.startTime)
	allAttrs := append(tc.attrs, attrs...)
	tc.timer.Record(tc.ctx, duration, allAttrs...)
}

type Histogram struct {
	histogram metric.Int64Histogram
	attrs     []attribute.KeyValue
}

type HistogramConfig struct {
	Name        string
	Description string
	Unit        string
	Attributes  map[string]string
	Boundaries  []float64
}

func NewHistogram(meter metric.Meter, cfg HistogramConfig) (*Histogram, error) {
	opts := []metric.Int64HistogramOption{
		metric.WithDescription(cfg.Description),
	}

	if cfg.Unit != "" {
		opts = append(opts, metric.WithUnit(cfg.Unit))
	}

	if len(cfg.Boundaries) > 0 {
		opts = append(opts, metric.WithExplicitBucketBoundaries(cfg.Boundaries...))
	}

	histogram, err := meter.Int64Histogram(cfg.Name, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create histogram %s: %w", cfg.Name, err)
	}

	attrs := make([]attribute.KeyValue, 0, len(cfg.Attributes))
	for k, v := range cfg.Attributes {
		attrs = append(attrs, attribute.String(k, v))
	}

	return &Histogram{
		histogram: histogram,
		attrs:     attrs,
	}, nil
}

func (h *Histogram) Record(ctx context.Context, value int64, attrs ...attribute.KeyValue) {
	allAttrs := append(h.attrs, attrs...)
	h.histogram.Record(ctx, value, metric.WithAttributes(allAttrs...))
}

func (h *Histogram) WithAttributes(attrs ...attribute.KeyValue) *Histogram {
	return &Histogram{
		histogram: h.histogram,
		attrs:     append(h.attrs, attrs...),
	}
}