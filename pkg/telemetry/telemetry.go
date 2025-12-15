package telemetry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/trace"
)

var log = logging.Logger("telemetry")

const (
	defaultEndpoint        = "telemetry.storacha.network:443"
	defaultPublishInterval = 30 * time.Second
)

type Telemetry struct {
	provider      *Provider
	traceProvider *trace.TracerProvider
	meter         metric.Meter
}

func New(ctx context.Context, cfg Config) (*Telemetry, error) {
	// collector endpoint and environment will be hard-coded for now
	cfg.endpoint = defaultEndpoint
	if cfg.PublishInterval == 0 {
		cfg.PublishInterval = defaultPublishInterval
	}

	// tracing is on by default, but will only sample if parent is sampled
	if cfg.TracesEndpoint == "" {
		if os.Getenv("PIRI_TRACING_ENABLED") != "0" {
			cfg.TracesEndpoint = cfg.endpoint
		}
	}

	res, err := newResource(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create telemetry resource: %w", err)
	}

	metricOpts := newOTLPHTTPOptions(cfg.endpoint, cfg.insecure, cfg.headers).metricOptions()
	provider, err := newProvider(ctx, cfg, res, metricOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create telemetry provider: %w", err)
	}

	t := &Telemetry{
		provider: provider,
		meter:    provider.Meter(),
	}

	traceOpts := newOTLPHTTPOptions(cfg.TracesEndpoint, cfg.insecure, cfg.headers).traceOptions()
	if len(traceOpts) > 0 {
		traceProvider, err := newTraceProvider(ctx, res, traceOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to create trace provider: %w", err)
		}
		t.traceProvider = traceProvider
	}
	return t, nil
}

// NewWithMeter creates a new Telemetry instance with a custom meter.
// This is useful for testing with in-memory exporters or manual readers.
func NewWithMeter(meter metric.Meter) *Telemetry {
	return &Telemetry{
		meter: meter,
	}
}

func (t *Telemetry) Meter() metric.Meter {
	return t.meter
}

func (t *Telemetry) NewCounter(cfg CounterConfig) (*Counter, error) {
	return NewCounter(t.meter, cfg)
}

func (t *Telemetry) NewFloatCounter(cfg FloatCounterConfig) (*FloatCounter, error) {
	return NewFloatCounter(t.meter, cfg)
}

func (t *Telemetry) NewGauge(cfg GaugeConfig) (*Gauge, error) {
	return NewGauge(t.meter, cfg)
}

func (t *Telemetry) NewFloatGauge(cfg FloatGaugeConfig) (*FloatGauge, error) {
	return NewFloatGauge(t.meter, cfg)
}

func (t *Telemetry) NewTimer(cfg TimerConfig) (*Timer, error) {
	return NewTimer(t.meter, cfg)
}

func (t *Telemetry) NewHistogram(cfg HistogramConfig) (*Histogram, error) {
	return NewHistogram(t.meter, cfg)
}

func (t *Telemetry) NewInfo(cfg InfoConfig) (*Info, error) {
	return NewInfo(t.meter, cfg)
}

func (t *Telemetry) ForceFlush(ctx context.Context) error {
	var errs []error

	if t.provider != nil {
		if err := t.provider.provider.ForceFlush(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if t.traceProvider != nil {
		if err := t.traceProvider.ForceFlush(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (t *Telemetry) Shutdown(ctx context.Context) error {
	var errs []error

	if t.provider != nil {
		if err := t.provider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if t.traceProvider != nil {
		if err := t.traceProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func StringAttr(key, value string) attribute.KeyValue {
	return attribute.String(key, value)
}

func IntAttr(key string, value int) attribute.KeyValue {
	return attribute.Int(key, value)
}

func Int64Attr(key string, value int64) attribute.KeyValue {
	return attribute.Int64(key, value)
}

func FloatAttr(key string, value float64) attribute.KeyValue {
	return attribute.Float64(key, value)
}

func BoolAttr(key string, value bool) attribute.KeyValue {
	return attribute.Bool(key, value)
}

const (
	KiB float64 = 1024
	MiB         = KiB * 1024
	GiB         = MiB * 1024
)

var SizeBoundaries = []float64{
	// Explicit histogram buckets for request/response body sizes (bytes), up to 1 GiB.
	KiB,
	2 * KiB,
	4 * KiB,
	8 * KiB,
	16 * KiB,
	32 * KiB,
	64 * KiB,
	128 * KiB,
	256 * KiB,
	512 * KiB,
	MiB,
	2 * MiB,
	4 * MiB,
	8 * MiB,
	16 * MiB,
	32 * MiB,
	64 * MiB,
	128 * MiB,
	256 * MiB,
	512 * MiB,
	GiB,
}
