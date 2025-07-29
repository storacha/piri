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
		counter, err := Global().NewCounter(CounterConfig{
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

	t.Run("Shutdown handles nil global", func(t *testing.T) {
		SetGlobalForTesting(nil)

		// Should not panic
		err := Shutdown(ctx)
		assert.NoError(t, err)
	})
}
