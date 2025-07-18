package telemetry_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"

	"github.com/storacha/piri/pkg/telemetry"
)

func TestMetricsWithManualReader(t *testing.T) {
	ctx := context.Background()
	reader := metric.NewManualReader()

	// Create a meter provider with manual reader
	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", "test-service"),
		),
	)
	require.NoError(t, err)

	provider := metric.NewMeterProvider(
		metric.WithReader(reader),
		metric.WithResource(res),
	)
	defer func() {
		err := provider.Shutdown(ctx)
		require.NoError(t, err)
	}()

	// Create telemetry with custom meter
	tel := telemetry.NewWithMeter(provider.Meter("test-service"))
	defer tel.Shutdown(ctx)

	t.Run("Counter", func(t *testing.T) {
		counter, err := tel.NewCounter(telemetry.CounterConfig{
			Name:        "test_counter",
			Description: "Test counter",
		})
		require.NoError(t, err)

		// Record some values
		counter.Add(ctx, 5)
		counter.Inc(ctx)
		counter.Add(ctx, 10, telemetry.StringAttr("method", "GET"))

		// Collect metrics
		rm := metricdata.ResourceMetrics{}
		err = reader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Find our metric
		var testMetric *metricdata.Metrics
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "test_counter" {
					testMetric = &m
					break
				}
			}
		}
		require.NotNil(t, testMetric)
		assert.Equal(t, "Test counter", testMetric.Description)

		// Check the data
		sum, ok := testMetric.Data.(metricdata.Sum[int64])
		assert.True(t, ok)
		assert.True(t, sum.IsMonotonic)
		assert.GreaterOrEqual(t, len(sum.DataPoints), 1)

		// Check total value
		var total int64
		for _, dp := range sum.DataPoints {
			total += dp.Value
		}
		assert.Equal(t, int64(16), total) // 5 + 1 + 10
	})

	t.Run("Gauge", func(t *testing.T) {
		gauge, err := tel.NewGauge(telemetry.GaugeConfig{
			Name:        "test_gauge",
			Description: "Test gauge",
			Unit:        "connections",
		})
		require.NoError(t, err)

		// Record some values
		gauge.Record(ctx, 100)
		gauge.Record(ctx, 50)
		gauge.Record(ctx, 75, telemetry.StringAttr("pool", "primary"))

		// Collect metrics
		rm := metricdata.ResourceMetrics{}
		err = reader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Find our metric
		var testMetric *metricdata.Metrics
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "test_gauge" {
					testMetric = &m
					break
				}
			}
		}
		require.NotNil(t, testMetric)
		assert.Equal(t, "connections", testMetric.Unit)

		// Check the data
		gaugeData, ok := testMetric.Data.(metricdata.Gauge[int64])
		assert.True(t, ok)
		assert.GreaterOrEqual(t, len(gaugeData.DataPoints), 1)

		// Check that we have the expected values
		hasDefault := false
		hasPrimary := false
		for _, dp := range gaugeData.DataPoints {
			attrs := dp.Attributes.ToSlice()
			if len(attrs) == 0 {
				hasDefault = true
				assert.Equal(t, int64(50), dp.Value) // Last value without attributes
			} else {
				for _, attr := range attrs {
					if attr.Key == "pool" && attr.Value.AsString() == "primary" {
						hasPrimary = true
						assert.Equal(t, int64(75), dp.Value)
					}
				}
			}
		}
		assert.True(t, hasDefault || hasPrimary)
	})

	t.Run("Timer", func(t *testing.T) {
		timer, err := tel.NewTimer(telemetry.TimerConfig{
			Name:        "test_timer",
			Description: "Test timer",
			Unit:        "ms",
			Boundaries:  []float64{10, 50, 100, 500, 1000},
		})
		require.NoError(t, err)

		// Record some durations
		timer.Record(ctx, 25*time.Millisecond)
		timer.Record(ctx, 150*time.Millisecond)
		timer.RecordFloat(ctx, 75.5)

		// Collect metrics
		rm := metricdata.ResourceMetrics{}
		err = reader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Find our metric
		var testMetric *metricdata.Metrics
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "test_timer" {
					testMetric = &m
					break
				}
			}
		}
		require.NotNil(t, testMetric)
		assert.Equal(t, "ms", testMetric.Unit)

		// Check the data
		hist, ok := testMetric.Data.(metricdata.Histogram[float64])
		assert.True(t, ok)
		assert.GreaterOrEqual(t, len(hist.DataPoints), 1)

		// Check aggregated data
		var totalCount uint64
		var totalSum float64
		for _, dp := range hist.DataPoints {
			totalCount += dp.Count
			totalSum += dp.Sum
		}
		assert.Equal(t, uint64(3), totalCount) // 3 recordings
		assert.Equal(t, 250.5, totalSum)       // 25 + 150 + 75.5
	})

	t.Run("TimedContext", func(t *testing.T) {
		timer, err := tel.NewTimer(telemetry.TimerConfig{
			Name: "test_timed_operation",
		})
		require.NoError(t, err)

		// Time an operation
		operation := timer.Start(ctx, telemetry.StringAttr("operation", "test"))
		time.Sleep(10 * time.Millisecond)
		operation.End(telemetry.BoolAttr("success", true))

		// Collect metrics
		rm := metricdata.ResourceMetrics{}
		err = reader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Find our metric
		var testMetric *metricdata.Metrics
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "test_timed_operation" {
					testMetric = &m
					break
				}
			}
		}
		require.NotNil(t, testMetric)

		hist, ok := testMetric.Data.(metricdata.Histogram[float64])
		assert.True(t, ok)
		assert.GreaterOrEqual(t, len(hist.DataPoints), 1)

		// Check that timing was recorded
		var found bool
		for _, dp := range hist.DataPoints {
			if dp.Count > 0 {
				found = true
				assert.GreaterOrEqual(t, dp.Sum, float64(10)) // At least 10ms

				// Check attributes
				attrs := dp.Attributes.ToSlice()
				hasOperation := false
				hasSuccess := false
				for _, attr := range attrs {
					if attr.Key == "operation" && attr.Value.AsString() == "test" {
						hasOperation = true
					}
					if attr.Key == "success" && attr.Value.AsBool() == true {
						hasSuccess = true
					}
				}
				assert.True(t, hasOperation)
				assert.True(t, hasSuccess)
			}
		}
		assert.True(t, found)
	})
}

func TestFloatMetrics(t *testing.T) {
	ctx := context.Background()
	reader := metric.NewManualReader()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", "test-service"),
		),
	)
	require.NoError(t, err)

	provider := metric.NewMeterProvider(
		metric.WithReader(reader),
		metric.WithResource(res),
	)
	defer provider.Shutdown(ctx)

	tel := telemetry.NewWithMeter(provider.Meter("test-service"))

	t.Run("FloatCounter", func(t *testing.T) {
		counter, err := tel.NewFloatCounter(telemetry.FloatCounterConfig{
			Name:        "test_float_counter",
			Description: "Test float counter",
			Unit:        "seconds",
		})
		require.NoError(t, err)

		// Record some values
		counter.Add(ctx, 3.14)
		counter.Add(ctx, 2.86)

		// Collect metrics
		rm := metricdata.ResourceMetrics{}
		err = reader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Verify the metric exists and has correct sum
		found := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "test_float_counter" {
					found = true
					sum, ok := m.Data.(metricdata.Sum[float64])
					assert.True(t, ok)
					assert.True(t, sum.IsMonotonic)

					var total float64
					for _, dp := range sum.DataPoints {
						total += dp.Value
					}
					assert.InDelta(t, 6.0, total, 0.01) // 3.14 + 2.86
				}
			}
		}
		assert.True(t, found)
	})

	t.Run("FloatGauge", func(t *testing.T) {
		gauge, err := tel.NewFloatGauge(telemetry.FloatGaugeConfig{
			Name:        "test_float_gauge",
			Description: "Test float gauge",
			Unit:        "%",
		})
		require.NoError(t, err)

		// Record some values
		gauge.Record(ctx, 75.5)
		gauge.Record(ctx, 82.3)

		// Collect metrics
		rm := metricdata.ResourceMetrics{}
		err = reader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Verify the metric exists
		found := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "test_float_gauge" {
					found = true
					gaugeData, ok := m.Data.(metricdata.Gauge[float64])
					assert.True(t, ok)
					assert.GreaterOrEqual(t, len(gaugeData.DataPoints), 1)

					// Should have the last recorded value
					for _, dp := range gaugeData.DataPoints {
						assert.Equal(t, 82.3, dp.Value)
					}
				}
			}
		}
		assert.True(t, found)
	})
}

func TestHistogramMetric(t *testing.T) {
	ctx := context.Background()
	reader := metric.NewManualReader()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", "test-service"),
		),
	)
	require.NoError(t, err)

	provider := metric.NewMeterProvider(
		metric.WithReader(reader),
		metric.WithResource(res),
	)
	defer provider.Shutdown(ctx)

	tel := telemetry.NewWithMeter(provider.Meter("test-service"))

	histogram, err := tel.NewHistogram(telemetry.HistogramConfig{
		Name:        "test_histogram",
		Description: "Test histogram",
		Unit:        "bytes",
		Boundaries:  []float64{1024, 2048, 4096, 8192},
	})
	require.NoError(t, err)

	// Record some values
	histogram.Record(ctx, 512)   // bucket 0
	histogram.Record(ctx, 1536)  // bucket 1
	histogram.Record(ctx, 3072)  // bucket 2
	histogram.Record(ctx, 10240) // bucket 4 (overflow)

	// Collect metrics
	rm := metricdata.ResourceMetrics{}
	err = reader.Collect(ctx, &rm)
	require.NoError(t, err)

	// Find and verify the metric
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "test_histogram" {
				found = true
				hist, ok := m.Data.(metricdata.Histogram[int64])
				assert.True(t, ok)
				assert.GreaterOrEqual(t, len(hist.DataPoints), 1)

				var totalCount uint64
				var totalSum int64
				for _, dp := range hist.DataPoints {
					totalCount += dp.Count
					totalSum += dp.Sum
				}
				assert.Equal(t, uint64(4), totalCount)
				assert.Equal(t, int64(15360), totalSum) // 512 + 1536 + 3072 + 10240
			}
		}
	}
	assert.True(t, found)
}

func TestInfo(t *testing.T) {
	ctx := context.Background()
	reader := metric.NewManualReader()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", "test-service"),
		),
	)
	require.NoError(t, err)

	provider := metric.NewMeterProvider(
		metric.WithReader(reader),
		metric.WithResource(res),
	)
	defer provider.Shutdown(ctx)

	tel := telemetry.NewWithMeter(provider.Meter("test-service"))

	t.Run("Info metric always records 1.0", func(t *testing.T) {
		info, err := tel.NewInfo(telemetry.InfoConfig{
			Name:        "test_info",
			Description: "Test info metric",
			Labels: map[string]string{
				"version": "v1.0.0",
				"commit":  "abc123",
			},
		})
		require.NoError(t, err)

		// Record the info
		info.Record(ctx)

		// Collect metrics
		rm := metricdata.ResourceMetrics{}
		err = reader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Find and verify the metric
		found := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "test_info" {
					found = true
					gauge, ok := m.Data.(metricdata.Gauge[float64])
					assert.True(t, ok)
					assert.Len(t, gauge.DataPoints, 1)

					dp := gauge.DataPoints[0]
					assert.Equal(t, 1.0, dp.Value)

					// Check labels
					attrs := dp.Attributes.ToSlice()
					assert.Len(t, attrs, 2)

					hasVersion := false
					hasCommit := false
					for _, attr := range attrs {
						if attr.Key == "version" && attr.Value.AsString() == "v1.0.0" {
							hasVersion = true
						}
						if attr.Key == "commit" && attr.Value.AsString() == "abc123" {
							hasCommit = true
						}
					}
					assert.True(t, hasVersion)
					assert.True(t, hasCommit)
				}
			}
		}
		assert.True(t, found)
	})

	t.Run("Info metric update", func(t *testing.T) {
		info, err := tel.NewInfo(telemetry.InfoConfig{
			Name:        "test_info_update",
			Description: "Test info metric update",
			Labels: map[string]string{
				"address": "0x1234",
				"network": "mainnet",
			},
		})
		require.NoError(t, err)

		// Record initial values
		info.Record(ctx)

		// Update with new values
		info.Update(ctx, map[string]string{
			"address": "0x5678",
			"network": "testnet",
		})

		// Collect metrics
		rm := metricdata.ResourceMetrics{}
		err = reader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Verify updated values
		found := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "test_info_update" {
					found = true
					gauge, ok := m.Data.(metricdata.Gauge[float64])
					assert.True(t, ok)

					// Find the data point with updated values
					foundUpdated := false
					for _, dp := range gauge.DataPoints {
						assert.Equal(t, 1.0, dp.Value)

						// Check if this is the updated data point
						attrs := dp.Attributes.ToSlice()
						addressCorrect := false
						networkCorrect := false

						for _, attr := range attrs {
							if attr.Key == "address" && attr.Value.AsString() == "0x5678" {
								addressCorrect = true
							}
							if attr.Key == "network" && attr.Value.AsString() == "testnet" {
								networkCorrect = true
							}
						}

						if addressCorrect && networkCorrect {
							foundUpdated = true
							break
						}
					}
					assert.True(t, foundUpdated, "Updated data point not found")
				}
			}
		}
		assert.True(t, found)
	})
}
