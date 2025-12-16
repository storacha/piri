package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Counter struct {
	counter metric.Int64Counter
}

func NewCounter(meter metric.Meter, name, description, unit string) (*Counter, error) {
	if name == "" {
		return nil, fmt.Errorf("counter name required")
	}
	if description == "" {
		return nil, fmt.Errorf("counter description required")
	}
	counter, err := meter.Int64Counter(
		name,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create counter %s: %w", name, err)
	}

	return &Counter{
		counter: counter,
	}, nil
}

func (c *Counter) Add(ctx context.Context, value int64, attrs ...attribute.KeyValue) {
	c.counter.Add(ctx, value, metric.WithAttributes(attrs...))
}

func (c *Counter) Inc(ctx context.Context, attrs ...attribute.KeyValue) {
	c.Add(ctx, 1, attrs...)
}

type UpDownCounter struct {
	counter metric.Int64UpDownCounter
}

func NewUpDownCounter(meter metric.Meter, name string, description string, unit string) (*UpDownCounter, error) {
	if name == "" {
		return nil, fmt.Errorf("counter name required")
	}
	if description == "" {
		return nil, fmt.Errorf("counter description required")
	}

	counter, err := meter.Int64UpDownCounter(
		name,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create counter %s: %w", name, err)
	}

	return &UpDownCounter{
		counter: counter,
	}, nil
}

func (c *UpDownCounter) Add(ctx context.Context, value int64, attrs ...attribute.KeyValue) {
	c.counter.Add(ctx, value, metric.WithAttributes(attrs...))
}

func (c *UpDownCounter) Inc(ctx context.Context, attrs ...attribute.KeyValue) {
	c.Add(ctx, 1, attrs...)
}

type Timer struct {
	histogram metric.Float64Histogram
}

func NewTimer(meter metric.Meter, name, description string, boundaries []float64) (*Timer, error) {
	if name == "" {
		return nil, fmt.Errorf("timer name required")
	}
	if description == "" {
		return nil, fmt.Errorf("timer description required")
	}
	if len(boundaries) == 0 {
		return nil, fmt.Errorf("timer boundaries required")
	}
	histogram, err := meter.Float64Histogram(
		name,
		metric.WithDescription(description),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(boundaries...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create timer %s: %w", name, err)
	}

	return &Timer{
		histogram: histogram,
	}, nil

}

func (t *Timer) Record(ctx context.Context, duration time.Duration, attrs ...attribute.KeyValue) {
	t.histogram.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs...))
}

func (t *Timer) Start(attrs ...attribute.KeyValue) *StopWatch {
	return &StopWatch{
		timer:     t,
		startTime: time.Now(),
		attrs:     attrs,
	}
}

func (t *StopWatch) Stop(ctx context.Context, attrs ...attribute.KeyValue) {
	duration := time.Since(t.startTime)
	allAttrs := append(t.attrs, attrs...)
	t.timer.Record(ctx, duration, allAttrs...)
}

type StopWatch struct {
	timer     *Timer
	startTime time.Time
	attrs     []attribute.KeyValue
}

// Info represents an info metric - a gauge that always has value 1
// Info metrics are used to expose textual information as labels.
type Info struct {
	gauge metric.Int64Gauge
	attrs []attribute.KeyValue
}

// NewInfo creates a new info metric. Under the hood it is a gauge that always reports 1.
// This is useful for exposing version info, addresses, and other metadata
func NewInfo(meter metric.Meter, name, description string, attrs ...attribute.KeyValue) (*Info, error) {
	gauge, err := meter.Int64Gauge(name, metric.WithDescription(description))
	if err != nil {
		return nil, err
	}

	return &Info{
		gauge: gauge,
		attrs: attrs,
	}, nil
}

// Record records the info metric with the given attributes, merging them with existing ones
func (i *Info) Record(ctx context.Context) {
	// Update the stored attributes
	i.gauge.Record(ctx, 1, metric.WithAttributes(i.attrs...))
}
