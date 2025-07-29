package telemetry

import (
	"context"
	"sync"

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

// setGlobalForTesting sets a custom telemetry instance for testing.
// This should only be used in tests.
func setGlobalForTesting(tel *Telemetry) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalTelemetry = tel
}
