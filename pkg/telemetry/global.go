package telemetry

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

var (
	globalTelemetry *Telemetry
	globalMu        sync.RWMutex
	globalOnce      sync.Once
)

// Initialize sets up the global telemetry instance.
// This should be called once at application startup.
func Initialize(ctx context.Context, cfg Config) error {
	globalMu.Lock()
	defer globalMu.Unlock()

	tel, err := New(ctx, cfg)
	if err != nil {
		return err
	}

	globalTelemetry = tel
	return nil
}

// Global returns the global telemetry instance.
// If Initialize hasn't been called, it returns a no-op telemetry instance.
func Global() *Telemetry {
	globalMu.RLock()
	if globalTelemetry != nil {
		defer globalMu.RUnlock()
		return globalTelemetry
	}
	globalMu.RUnlock()

	// If not initialized, create a no-op instance
	globalOnce.Do(func() {
		globalMu.Lock()
		defer globalMu.Unlock()
		if globalTelemetry == nil {
			globalTelemetry = NewWithMeter(noop.NewMeterProvider().Meter("noop"))
		}
	})

	return globalTelemetry
}

// Shutdown shuts down the global telemetry instance.
func Shutdown(ctx context.Context) error {
	globalMu.RLock()
	defer globalMu.RUnlock()

	if globalTelemetry != nil {
		return globalTelemetry.Shutdown(ctx)
	}
	return nil
}

// MustInitialize is like Initialize but panics on error.
// Useful for application startup where telemetry is critical.
func MustInitialize(ctx context.Context, cfg Config) {
	if err := Initialize(ctx, cfg); err != nil {
		panic(err)
	}
}

// GlobalCounter returns a new counter using the global telemetry instance.
func GlobalCounter(cfg CounterConfig) (*Counter, error) {
	return Global().NewCounter(cfg)
}

// GlobalFloatCounter returns a new float counter using the global telemetry instance.
func GlobalFloatCounter(cfg FloatCounterConfig) (*FloatCounter, error) {
	return Global().NewFloatCounter(cfg)
}

// GlobalGauge returns a new gauge using the global telemetry instance.
func GlobalGauge(cfg GaugeConfig) (*Gauge, error) {
	return Global().NewGauge(cfg)
}

// GlobalFloatGauge returns a new float gauge using the global telemetry instance.
func GlobalFloatGauge(cfg FloatGaugeConfig) (*FloatGauge, error) {
	return Global().NewFloatGauge(cfg)
}

// GlobalTimer returns a new timer using the global telemetry instance.
func GlobalTimer(cfg TimerConfig) (*Timer, error) {
	return Global().NewTimer(cfg)
}

// GlobalHistogram returns a new histogram using the global telemetry instance.
func GlobalHistogram(cfg HistogramConfig) (*Histogram, error) {
	return Global().NewHistogram(cfg)
}

// GlobalInfo returns a new info metric using the global telemetry instance.
func GlobalInfo(cfg InfoConfig) (*Info, error) {
	return Global().NewInfo(cfg)
}

// Meter returns the meter from the global telemetry instance.
func Meter() metric.Meter {
	return Global().Meter()
}

// SetGlobalForTesting sets a custom telemetry instance for testing.
// This should only be used in tests.
func SetGlobalForTesting(tel *Telemetry) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalTelemetry = tel
}
