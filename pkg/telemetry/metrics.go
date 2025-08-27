package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/storacha/piri/pkg/build"
)

const meterName = "github.com/storacha/piri"

var (
	//meter metric.meter

	// HTTP metrics
	HTTPRequestDuration metric.Float64Histogram
	HTTPRequestsTotal   metric.Int64Counter
	HTTPRequestSize     metric.Float64Histogram
	HTTPResponseSize    metric.Float64Histogram

	// Task metrics
	TaskExecutionDuration metric.Float64Histogram
	TasksTotal            metric.Int64Counter
	TaskQueueDepth        metric.Int64UpDownCounter
	TaskRetriesTotal      metric.Int64Counter

	// PDP metrics
	ProofsSubmitted         metric.Int64Counter
	ProofsFailed            metric.Int64Counter
	ProofSetCount           metric.Int64UpDownCounter
	NextProofDeadline       metric.Float64Gauge
	ChallengeWindowDuration metric.Float64Gauge
	RootsTotal              metric.Int64Gauge
	PDPDataSize             metric.Int64Gauge

	// Storage metrics
	StorageUsed              metric.Int64Gauge
	PiecesStored             metric.Int64Gauge
	StashFilesCount          metric.Int64Gauge
	StorageOperationsTotal   metric.Int64Counter
	StorageOperationDuration metric.Float64Histogram

	// Aggregation metrics
	AggregatesCreated       metric.Int64Counter
	AggregateSize           metric.Int64Histogram
	AggregationDuration     metric.Float64Histogram
	AggregationPiecesQueued metric.Int64UpDownCounter

	// Ethereum metrics
	EthTransactionsTotal           metric.Int64Counter
	EthTransactionGasUsed          metric.Float64Histogram
	EthTransactionConfirmationTime metric.Float64Histogram
)

// SetupMetrics sets up OpenTelemetry metrics and the Prometheus exporter.
// If setup fails, the process logs and exits.
func SetupMetrics(ctx context.Context) *prometheus.Exporter {
	exporter, err := prometheus.New()
	if err != nil {
		log.Fatal(err)
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
	)
	meter := provider.Meter(meterName)

	// HTTP metrics
	HTTPRequestDuration, err = meter.Float64Histogram(
		"http.server.duration.seconds",
		metric.WithDescription("Duration of HTTP requests in seconds, by endpoint, method, and status"),
	)
	if err != nil {
		log.Fatal(err)
	}

	HTTPRequestsTotal, err = meter.Int64Counter(
		"http.server.requests.count",
		metric.WithDescription("Total number of HTTP requests, by endpoint, method, and status"),
	)
	if err != nil {
		log.Fatal(err)
	}

	HTTPRequestSize, err = meter.Float64Histogram(
		"http.server.request.size.bytes",
		metric.WithDescription("Size of HTTP request bodies in bytes"),
	)
	if err != nil {
		log.Fatal(err)
	}

	HTTPResponseSize, err = meter.Float64Histogram(
		"http.server.response.size.bytes",
		metric.WithDescription("Size of HTTP response bodies in bytes"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Task metrics
	TaskExecutionDuration, err = meter.Float64Histogram(
		"task.execution.duration.seconds",
		metric.WithDescription("Task execution time in seconds by task_name and status"),
	)
	if err != nil {
		log.Fatal(err)
	}

	TasksTotal, err = meter.Int64Counter(
		"task.execution.count",
		metric.WithDescription("Total number of tasks executed by task_name and status"),
	)
	if err != nil {
		log.Fatal(err)
	}

	TaskQueueDepth, err = meter.Int64UpDownCounter(
		"task.queue.depth.count",
		metric.WithDescription("Current task queue depth by queue_name"),
	)
	if err != nil {
		log.Fatal(err)
	}

	TaskRetriesTotal, err = meter.Int64Counter(
		"task.retries.count",
		metric.WithDescription("Total number of task retries attempted"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// PDP metrics
	ProofsSubmitted, err = meter.Int64Counter(
		"pdp.proofs.submitted.count",
		metric.WithDescription("Total successful proof submissions"),
	)
	if err != nil {
		log.Fatal(err)
	}

	ProofsFailed, err = meter.Int64Counter(
		"pdp.proofs.failed.count",
		metric.WithDescription("Total failed proof submissions by reason"),
	)
	if err != nil {
		log.Fatal(err)
	}

	ProofSetCount, err = meter.Int64UpDownCounter("pdp.proofsets.active.count",
		metric.WithDescription("Number of active proof sets"),
	)
	if err != nil {
		log.Fatal(err)
	}

	NextProofDeadline, err = meter.Float64Gauge(
		"pdp.proofs.deadline.seconds",
		metric.WithDescription("Time until next proof deadline in seconds"),
	)
	if err != nil {
		log.Fatal(err)
	}

	ChallengeWindowDuration, err = meter.Float64Gauge(
		"pdp.challenge.window.duration.seconds",
		metric.WithDescription("Challenge window duration in seconds"),
	)
	if err != nil {
		log.Fatal(err)
	}

	RootsTotal, err = meter.Int64Gauge(
		"pdp.roots.count",
		metric.WithDescription("Total number of roots across all proof sets"),
	)
	if err != nil {
		log.Fatal(err)
	}

	PDPDataSize, err = meter.Int64Gauge(
		"pdp.data.size.bytes",
		metric.WithDescription("Total data size being proven in bytes"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Storage metrics
	StorageUsed, err = meter.Int64Gauge(
		"storage.used.bytes",
		metric.WithDescription("Total storage used by type (blob, stash) in bytes"),
	)
	if err != nil {
		log.Fatal(err)
	}

	PiecesStored, err = meter.Int64Gauge(
		"storage.pieces.count",
		metric.WithDescription("Total number of pieces stored"),
	)
	if err != nil {
		log.Fatal(err)
	}

	StashFilesCount, err = meter.Int64Gauge(
		"storage.stash.files.count",
		metric.WithDescription("Total number of stash files stored"),
	)
	if err != nil {
		log.Fatal(err)
	}

	StorageOperationsTotal, err = meter.Int64Counter(
		"storage.operations.count",
		metric.WithDescription("Total storage operations by type and status"),
	)
	if err != nil {
		log.Fatal(err)
	}

	StorageOperationDuration, err = meter.Float64Histogram(
		"storage.operation.duration.seconds",
		metric.WithDescription("Latency of storage operations in seconds"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Aggregation metrics
	AggregatesCreated, err = meter.Int64Counter(
		"aggregation.created.count",
		metric.WithDescription("Total number of aggregates created"),
	)
	if err != nil {
		log.Fatal(err)
	}

	AggregateSize, err = meter.Int64Histogram(
		"aggregation.size.bytes",
		metric.WithDescription("Size distribution of aggregates in bytes"),
	)
	if err != nil {
		log.Fatal(err)
	}

	AggregationDuration, err = meter.Float64Histogram(
		"aggregation.duration.seconds",
		metric.WithDescription("Time taken to create aggregates in seconds"),
	)
	if err != nil {
		log.Fatal(err)
	}

	AggregationPiecesQueued, err = meter.Int64UpDownCounter(
		"aggregation.pieces.queued.count",
		metric.WithDescription("Number of pieces waiting for aggregation"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Ethereum metrics
	EthTransactionsTotal, err = meter.Int64Counter(
		"eth.transactions.count",
		metric.WithDescription("Total Ethereum transactions by type and status"),
	)
	if err != nil {
		log.Fatal(err)
	}

	EthTransactionGasUsed, err = meter.Float64Histogram(
		"eth.transaction.gas.used",
		metric.WithDescription("Gas used per Ethereum transaction"),
	)
	if err != nil {
		log.Fatal(err)
	}

	EthTransactionConfirmationTime, err = meter.Float64Histogram(
		"eth.transaction.confirmation.time.seconds",
		metric.WithDescription("Time taken for Ethereum transactions to confirm"),
	)
	if err != nil {
		log.Fatal(err)
	}

	return exporter
}

// RecordServerInfo records server metadata as an info metric
// this metric is best effort, if it fails, a warning is log and the process continues
func RecordServerInfo(ctx context.Context, serverType string, extraAttrs ...attribute.KeyValue) {
	info, err := Global().NewInfo(InfoConfig{
		Name:        "piri_server_info",
		Description: "Build and runtime information about the Piri server",
	})
	if err != nil {
		log.Warnw("failed to initialize piri server info metric", "error", err, "type", serverType)
	}

	// Base attributes that all servers share
	attrs := []attribute.KeyValue{
		StringAttr("version", build.Version),
		StringAttr("commit", build.Commit),
		StringAttr("built_by", build.BuiltBy),
		StringAttr("build_date", build.Date),
		Int64Attr("start_time_unix", time.Now().Unix()),
		StringAttr("server_type", serverType),
	}

	// Add any server-specific attributes
	attrs = append(attrs, extraAttrs...)

	info.Record(ctx, attrs...)
}
