package telemetry

import (
	"context"
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var log = logging.Logger("telemetry")

const (
	defaultEndpoint        = "telemetry.storacha.network:443"
	defaultEnvironment     = "warm-staging"
	defaultPublishInterval = 30 * time.Second
)

type Telemetry struct {
	provider *Provider
	meter    metric.Meter
}

func New(ctx context.Context, cfg Config) (*Telemetry, error) {
	// collector endpoint and environment will be hard-coded for now
	cfg.endpoint = defaultEndpoint
	cfg.environment = defaultEnvironment
	if cfg.PublishInterval == 0 {
		cfg.PublishInterval = defaultPublishInterval
	}

	provider, err := newProvider(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create telemetry provider: %w", err)
	}

	return &Telemetry{
		provider: provider,
		meter:    provider.Meter(),
	}, nil
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

func (t *Telemetry) Shutdown(ctx context.Context) error {
	if t.provider != nil {
		return t.provider.Shutdown(ctx)
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

var DefaultBoundaries = []float64{
	0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 20, 50, 100,
}

var LatencyBoundaries = []float64{
	0.1, 0.5, 1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000, 10000,
}

var SizeBoundaries = []float64{
	1024, 10240, 102400, 1048576, 10485760, 104857600, 1073741824,
}
