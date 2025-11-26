package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobalTelemetry(t *testing.T) {
	ctx := context.Background()

	t.Run("Global returns noop before initialization", func(t *testing.T) {
		// Reset global state for test
		setGlobalForTesting(nil)

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
		// Reset global state for test
		setGlobalForTesting(nil)

		// Before initialization, should get a no-op instance
		before := Global()
		assert.NotNil(t, before)

		// After initialization, should get the new instance
		Initialize(context.Background(), Config{
			ServiceName:    "test",
			ServiceVersion: "1.0.0",
			Environment:    "test",
			Endpoint:       "http://localhost:3000",
			endpoint:       "http://localhost:4317",
			insecure:       true,
		})
		after := Global()
		assert.NotEqual(t, before, after)
	})

	t.Run("Shutdown handles nil global", func(t *testing.T) {
		setGlobalForTesting(nil)

		// Should not panic
		err := Shutdown(ctx)
		assert.NoError(t, err)
	})
}
