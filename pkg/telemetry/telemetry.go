// Package telemetry provides utilities for creating and managing OpenTelemetry metrics.
//
// # Features
//
//   - **Counters**: Track monotonically increasing values (e.g., request counts, processed items)
//   - **Gauges**: Track values that can go up and down (e.g., active connections, CPU usage)
//   - **Timers/Histograms**: Track distributions of values (e.g., request latencies, response sizes)
//   - **OpenTelemetry Integration**: Exports metrics to any OpenTelemetry-compatible collector
//   - **Attribute Support**: Add contextual metadata to all metrics
//   - **Type Safety**: Strongly typed configuration and metric creation
//
// # Quick Start
//
// ## Option 1: Dependency Injection (Recommended for testing)
//
//	ctx := context.Background()
//	tel, err := telemetry.New(ctx, telemetry.Config{
//	    ServiceName:    "my-service",
//	    ServiceVersion: "1.0.0",
//	    Environment:    "production",
//	    Endpoint:       "localhost:4317",  // OpenTelemetry collector endpoint
//	    Insecure:       true,              // Use for local development
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer tel.Shutdown(ctx)
//
// ## Option 2: Global Instance (Simpler, less testable)
//
//	telemetry.InitGlobal(telemetry.Config{...})
//	defer telemetry.ShutdownGlobal()
//
// # Metric Types
//
// ## Counters
//
// Counters track monotonically increasing values:
//
//	counter, _ := tel.NewCounter(telemetry.CounterConfig{
//	    Name:        "requests_total",
//	    Description: "Total number of requests",
//	})
//	counter.Add(ctx, 1, telemetry.StringAttr("endpoint", "/api"))
//
// ## Gauges
//
// Gauges track values that can go up and down:
//
//	gauge, _ := tel.NewGauge(telemetry.GaugeConfig{
//	    Name:        "active_connections",
//	    Description: "Number of active connections",
//	})
//	gauge.Set(ctx, 42, telemetry.StringAttr("server", "web-1"))
//
// ## Timers
//
// Timers measure duration of operations:
//
//	timer, _ := tel.NewTimer(telemetry.TimerConfig{
//	    Name:        "request_duration",
//	    Description: "Request processing time",
//	    Unit:        "ms",
//	})
//
//	// Method 1: Manual timing
//	start := time.Now()
//	// ... do work ...
//	elapsed := time.Since(start)
//	timer.Record(ctx, elapsed, telemetry.StringAttr("endpoint", "/api"))
//
//	// Method 2: Automatic timing
//	operation := timer.Start(ctx, telemetry.StringAttr("operation", "database_query"))
//	defer operation.End()
//
// ## Histograms
//
// Histograms track the distribution of arbitrary values:
//
//	histogram, _ := tel.NewHistogram(telemetry.HistogramConfig{
//	    Name:        "response_size",
//	    Description: "Response size in bytes",
//	    Unit:        "bytes",
//	    Boundaries:  telemetry.SizeBoundaries,  // Predefined size boundaries
//	})
//	histogram.Record(ctx, responseSize, telemetry.StringAttr("endpoint", "/api"))
//
// # Complete HTTP Server Example
//
//	func main() {
//	    ctx := context.Background()
//
//	    // Initialize telemetry
//	    tel, err := telemetry.New(ctx, telemetry.Config{
//	        ServiceName:    "my-http-server",
//	        ServiceVersion: "1.0.0",
//	        Environment:    "production",
//	    })
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    defer tel.Shutdown(ctx)
//
//	    // Create metrics
//	    requestCounter, _ := tel.NewCounter(telemetry.CounterConfig{
//	        Name:        "http_requests_total",
//	        Description: "Total number of HTTP requests",
//	    })
//
//	    requestTimer, _ := tel.NewTimer(telemetry.TimerConfig{
//	        Name:        "http_request_duration_seconds",
//	        Description: "HTTP request duration in seconds",
//	    })
//
//	    // HTTP handler
//	    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
//	        start := time.Now()
//
//	        // Increment request counter
//	        requestCounter.Add(r.Context(), 1,
//	            telemetry.StringAttr("method", r.Method),
//	            telemetry.StringAttr("path", r.URL.Path),
//	        )
//
//	        // Your handler logic here
//	        time.Sleep(100 * time.Millisecond) // Simulate work
//	        w.Write([]byte("Hello, World!"))
//
//	        // Record request duration
//	        elapsed := time.Since(start)
//	        requestTimer.Record(r.Context(), elapsed,
//	            telemetry.StringAttr("method", r.Method),
//	            telemetry.StringAttr("path", r.URL.Path),
//	            telemetry.IntAttr("status", 200),
//	        )
//	    })
//
//	    log.Println("Server starting on :8080")
//	    log.Fatal(http.ListenAndServe(":8080", nil))
//	}
//
// # Attributes
//
// Add contextual information to metrics using attributes:
//
//	telemetry.StringAttr("endpoint", "/api")
//	telemetry.IntAttr("status_code", 200)
//	telemetry.BoolAttr("success", true)
//
// # Predefined Boundaries
//
// The package provides predefined boundary values for common use cases:
//
//	telemetry.DefaultBoundaries  // General purpose boundaries
//	telemetry.LatencyBoundaries  // For request latencies (seconds)
//	telemetry.SizeBoundaries     // For response/request sizes (bytes)
package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Telemetry struct {
	provider *Provider
	meter    metric.Meter
}

func New(ctx context.Context, cfg Config) (*Telemetry, error) {
	provider, err := NewProvider(ctx, cfg)
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

func (t *Telemetry) NewConstantGauge(cfg ConstantGaugeConfig) (*ConstantGauge, error) {
	return NewConstantGauge(t.meter, cfg)
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
