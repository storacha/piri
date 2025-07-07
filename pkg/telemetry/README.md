# Telemetry Package

This package provides utilities for creating and managing OpenTelemetry metrics including counters, gauges, and timers/histograms.

## Features

- **Counters**: Track monotonically increasing values (e.g., request counts, processed items)
- **Gauges**: Track values that can go up and down (e.g., active connections, CPU usage)
- **Timers/Histograms**: Track distributions of values (e.g., request latencies, response sizes)
- **OpenTelemetry Integration**: Exports metrics to any OpenTelemetry-compatible collector
- **Attribute Support**: Add contextual metadata to all metrics
- **Type Safety**: Strongly typed configuration and metric creation

## Quick Start

### Option 1: Dependency Injection (Recommended for testing)

```go
import (
    "context"
    "github.com/storacha/piri/pkg/telemetry"
)

// Initialize telemetry
ctx := context.Background()
tel, err := telemetry.New(ctx, telemetry.Config{
    ServiceName:    "my-service",
    ServiceVersion: "1.0.0",
    Environment:    "production",
    Endpoint:       "localhost:4317",  // OpenTelemetry collector endpoint
    Insecure:       true,              // Use insecure connection for local development
})
if err != nil {
    log.Fatal(err)
}
defer tel.Shutdown(ctx)

// Pass tel to services that need metrics
service := NewMyService(tel)
```

### Option 2: Global Instance (Simpler, less testable)

```go
import (
    "context"
    "github.com/storacha/piri/pkg/telemetry"
)

// In main.go
func main() {
    ctx := context.Background()
    
    // Initialize global telemetry once
    telemetry.MustInitialize(ctx, telemetry.Config{
        ServiceName: "my-service",
        Endpoint:    "localhost:4317",
        Insecure:    true,
    })
    defer telemetry.Shutdown(ctx)
    
    // Start your application
    runApp()
}

// Anywhere in your code
func someFunction() {
    // Create metrics without passing telemetry
    counter, _ := telemetry.GlobalCounter(telemetry.CounterConfig{
        Name:        "requests_total",
        Description: "Total number of requests",
    })
    
    counter.Inc(context.Background(), telemetry.StringAttr("method", "GET"))
}
```

## Configuration

The `Config` struct supports the following options:

- `ServiceName`: Name of your service (required)
- `ServiceVersion`: Version of your service
- `Environment`: Deployment environment (e.g., "production", "staging")
- `Endpoint`: OpenTelemetry collector endpoint (required)
- `Insecure`: Whether to use an insecure connection
- `Headers`: Optional headers to send with metrics

## Metric Types

### Info Metrics

Info metrics are used to expose metadata where the label values are more important than the numeric value. They always record a value of 1.0.

```go
// Expose node metadata
nodeInfo, _ := tel.NewInfo(telemetry.InfoConfig{
    Name:        "node_info",
    Description: "Node metadata information",
    Labels: map[string]string{
        "filecoin_address": "f1234567890abcdef",
        "node_version":     "v1.2.3",
        "network":          "mainnet",
    },
})

// Record the info (always records 1.0)
nodeInfo.Record(ctx)

// Update labels when they change
nodeInfo.Update(ctx, map[string]string{
    "filecoin_address": "f0987654321fedcba",
    "node_version":     "v1.2.4",
    "network":          "mainnet",
})
```

### Constant Gauges

Constant gauges are used for configuration values that rarely change but should be exposed as metrics.

```go
// Expose configuration constants
challengeWindow, _ := tel.NewConstantGauge(telemetry.ConstantGaugeConfig{
    Name:        "contract_challenge_window_blocks",
    Description: "Challenge window duration in blocks",
    Unit:        "blocks",
    Value:       100,
    Labels: map[string]string{
        "environment": "production",
    },
})

// Record the constant value
challengeWindow.Record(ctx)

// Update if the value changes
challengeWindow.UpdateValue(ctx, 150)
```

### Counters

Counters track values that only increase over time.

```go
counter, _ := tel.NewCounter(telemetry.CounterConfig{
    Name:        "processed_items",
    Description: "Number of items processed",
    Unit:        "1",  // Unit of measurement
    Attributes: map[string]string{
        "service": "processor",
    },
})

// Increment by 1
counter.Inc(ctx)

// Add custom value
counter.Add(ctx, 5)

// Add with attributes
counter.Add(ctx, 10, 
    telemetry.StringAttr("type", "document"),
    telemetry.BoolAttr("cached", true),
)
```

### Gauges

Gauges track values that can increase or decrease.

```go
gauge, _ := tel.NewGauge(telemetry.GaugeConfig{
    Name:        "active_connections",
    Description: "Number of active connections",
    Unit:        "connections",
})

// Record current value
gauge.Record(ctx, 42)

// Record with attributes
gauge.Record(ctx, 10, telemetry.StringAttr("pool", "primary"))
```

### Timers

Timers track the distribution of durations.

```go
timer, _ := tel.NewTimer(telemetry.TimerConfig{
    Name:        "request_duration",
    Description: "Request processing time",
    Unit:        "ms",
    Boundaries:  telemetry.LatencyBoundaries,  // Predefined bucket boundaries
})

// Method 1: Record duration directly
timer.Record(ctx, 250*time.Millisecond)

// Method 2: Automatic timing
operation := timer.Start(ctx, telemetry.StringAttr("operation", "database_query"))
// ... do work ...
operation.End(telemetry.BoolAttr("success", true))
```

### Histograms

Histograms track the distribution of arbitrary values.

```go
histogram, _ := tel.NewHistogram(telemetry.HistogramConfig{
    Name:        "response_size",
    Description: "Response size in bytes",
    Unit:        "bytes",
    Boundaries:  telemetry.SizeBoundaries,  // Predefined size boundaries
})

histogram.Record(ctx, 1024)
```

## Attributes

Add contextual information to metrics using attributes:

```go
// Attribute helper functions
telemetry.StringAttr("key", "value")
telemetry.IntAttr("count", 42)
telemetry.Int64Attr("large", 9223372036854775807)
telemetry.FloatAttr("ratio", 0.95)
telemetry.BoolAttr("enabled", true)
```

## Predefined Boundaries

The package includes predefined histogram boundaries for common use cases:

- `telemetry.DefaultBoundaries`: General purpose boundaries
- `telemetry.LatencyBoundaries`: Optimized for latency measurements (ms)
- `telemetry.SizeBoundaries`: Optimized for size measurements (bytes)

## Best Practices

1. **Initialize Once**: Create the telemetry provider once at application startup
2. **Reuse Metrics**: Create metric instances once and reuse them throughout your application
3. **Use Attributes**: Add relevant attributes to provide context for your metrics
4. **Choose Appropriate Types**: 
   - Use counters for totals
   - Use gauges for current values that can go up and down
   - Use timers/histograms for distributions
   - Use info metrics for metadata (versions, addresses, identities)
   - Use constant gauges for configuration values
5. **Set Units**: Always specify units for clarity (e.g., "ms", "bytes", "requests")
6. **Graceful Shutdown**: Always call `Shutdown()` to ensure metrics are flushed

### Best Practices for Info Metrics

Info metrics are perfect for exposing:
- Service version and build information
- Network addresses (Filecoin addresses, Ethereum contract addresses)
- Node identities and peer IDs
- Configuration that affects behavior

Example query in Prometheus to see all nodes and their versions:
```promql
node_info{job="piri"}
```

### Best Practices for Constant Gauges

Constant gauges are ideal for:
- Configuration parameters (challenge windows, timeouts, limits)
- ProofSet IDs and other semi-static identifiers
- Resource quotas and limits
- Contract parameters

These metrics make it easy to correlate behavior with configuration and detect when configuration changes occur.

## Example: HTTP Server Metrics

```go
type Server struct {
    requestCount    *telemetry.Counter
    activeRequests  *telemetry.Gauge
    requestDuration *telemetry.Timer
}

func (s *Server) HandleRequest(w http.ResponseWriter, r *http.Request) {
    // Start timing
    timer := s.requestDuration.Start(r.Context(),
        telemetry.StringAttr("method", r.Method),
        telemetry.StringAttr("path", r.URL.Path),
    )
    
    // Track active requests
    s.activeRequests.Record(r.Context(), 1)
    defer func() {
        s.activeRequests.Record(r.Context(), -1)
    }()
    
    // Increment counter
    s.requestCount.Inc(r.Context(),
        telemetry.StringAttr("method", r.Method),
        telemetry.StringAttr("path", r.URL.Path),
    )
    
    // ... handle request ...
    
    // End timing
    timer.End(telemetry.IntAttr("status", 200))
}
```