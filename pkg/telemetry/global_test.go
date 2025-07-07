package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestGlobalTelemetry(t *testing.T) {
	ctx := context.Background()

	t.Run("Global returns noop before initialization", func(t *testing.T) {
		// Reset global state for test
		SetGlobalForTesting(nil)
		
		// Before initialization, should get a no-op instance
		tel := Global()
		assert.NotNil(t, tel)
		
		// Should be able to create metrics without error
		counter, err := GlobalCounter(CounterConfig{
			Name: "test_counter",
		})
		require.NoError(t, err)
		assert.NotNil(t, counter)
		
		// Should be safe to use
		counter.Inc(ctx)
	})

	t.Run("Initialize sets global instance", func(t *testing.T) {
		// Create a test instance
		testTel := NewWithMeter(noop.NewMeterProvider().Meter("test"))
		SetGlobalForTesting(testTel)
		
		// Global should return our test instance
		global := Global()
		assert.Equal(t, testTel, global)
	})

	t.Run("Global functions use global instance", func(t *testing.T) {
		// Set a test instance
		testTel := NewWithMeter(noop.NewMeterProvider().Meter("test"))
		SetGlobalForTesting(testTel)
		
		// Create metrics using global functions
		counter, err := GlobalCounter(CounterConfig{
			Name: "global_counter",
		})
		require.NoError(t, err)
		assert.NotNil(t, counter)
		
		gauge, err := GlobalGauge(GaugeConfig{
			Name: "global_gauge",
		})
		require.NoError(t, err)
		assert.NotNil(t, gauge)
		
		timer, err := GlobalTimer(TimerConfig{
			Name: "global_timer",
		})
		require.NoError(t, err)
		assert.NotNil(t, timer)
		
		info, err := GlobalInfo(InfoConfig{
			Name: "global_info",
			Labels: map[string]string{
				"version": "test",
			},
		})
		require.NoError(t, err)
		assert.NotNil(t, info)
	})

	t.Run("Meter returns global meter", func(t *testing.T) {
		testTel := NewWithMeter(noop.NewMeterProvider().Meter("test"))
		SetGlobalForTesting(testTel)
		
		meter := Meter()
		assert.NotNil(t, meter)
		assert.Equal(t, testTel.Meter(), meter)
	})

	t.Run("Shutdown handles nil global", func(t *testing.T) {
		SetGlobalForTesting(nil)
		
		// Should not panic
		err := Shutdown(ctx)
		assert.NoError(t, err)
	})
}