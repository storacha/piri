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

func RecordHTTPRequest(ctx context.Context, method, route, urlPath string, statusCode int, duration time.Duration, reqSize, respSize int64) {
	opts := metric.WithAttributes(
		attribute.String("http.method", method),
		attribute.String("http.route", route),
		attribute.String("url.path", urlPath),
		attribute.Int("http.status_code", statusCode),
	)

	HTTPRequestDuration.Record(ctx, duration.Seconds())
	HTTPRequestsTotal.Add(ctx, 1, opts)
	if reqSize > 0 {
		HTTPRequestSize.Record(ctx, float64(reqSize))
	}
	if respSize > 0 {
		HTTPResponseSize.Record(ctx, float64(respSize))
	}
}

// Task Metric Helpers

func RecordTaskExecution(ctx context.Context, taskName, status string, duration time.Duration) {
	opts := metric.WithAttributes(
		attribute.String("task.name", taskName),
		attribute.String("task.status", status),
	)

	TaskExecutionDuration.Record(ctx, duration.Seconds(), opts)
	TasksTotal.Add(ctx, 1, opts)
}

func ObserveTaskQueueDepth(ctx context.Context, queueName string, depth int64) {
	opts := metric.WithAttributes(
		attribute.String("task.queue", queueName),
	)

	TaskQueueDepth.Add(ctx, depth, opts)
}

func IncTaskRetries(ctx context.Context, taskName string) {
	opts := metric.WithAttributes(
		attribute.String("task.name", taskName),
	)

	TaskRetriesTotal.Add(ctx, 1, opts)
}

// PDP Metric Helpers

func IncProofSubmitted(ctx context.Context) {
	ProofsSubmitted.Add(ctx, 1)
}

func IncProofFailed(ctx context.Context, reason string) {
	opts := metric.WithAttributes(
		attribute.String("pdp.reason", reason),
	)

	ProofsFailed.Add(ctx, 1, opts)
}

func AdjustProofSetCount(ctx context.Context, count int64) {
	ProofSetCount.Add(ctx, count)
}

func SetNextProofDeadline(ctx context.Context, seconds float64) {
	NextProofDeadline.Record(ctx, seconds)
}

func SetChallengeWindowDuration(ctx context.Context, seconds float64) {
	ChallengeWindowDuration.Record(ctx, seconds)
}

func SetRootsTotal(ctx context.Context, count int64) {
	RootsTotal.Record(ctx, count)
}

func SetPDPDataSize(ctx context.Context, size int64) {
	PDPDataSize.Record(ctx, size)
}

// Storage metric helpers

func RecordStorageExecution(ctx context.Context, opType, status string, duration time.Duration) {
	opts := metric.WithAttributes(
		attribute.String("storage.operation", opType),
		attribute.String("storage.status", status),
	)

	StorageOperationDuration.Record(ctx, duration.Seconds(), opts)
	StorageOperationsTotal.Add(ctx, 1, opts)
}

func RecordStorageUsage(ctx context.Context, storageType string, usage int64) {
	opts := metric.WithAttributes(
		attribute.String("storage.type", storageType),
	)
	StorageUsed.Record(ctx, usage, opts)
}

func RecordPiecesStored(ctx context.Context, storageType string, count int64) {
	opts := metric.WithAttributes(
		attribute.String("storage.type", storageType),
	)

	PiecesStored.Record(ctx, count, opts)
}

func RecordStashFiles(ctx context.Context, count int64, storageType string) {
	opts := metric.WithAttributes(
		attribute.String("storage.type", storageType),
	)

	StashFilesCount.Record(ctx, count, opts)
}

// Aggregation metric helpers

func IncAggregatesCreated(ctx context.Context) {
	AggregatesCreated.Add(ctx, 1)
}

func RecordAggregateSize(ctx context.Context, sizeBytes int64) {
	AggregateSize.Record(ctx, sizeBytes)
}

func RecordAggregationDuration(ctx context.Context, duration time.Duration) {
	AggregationDuration.Record(ctx, duration.Seconds())
}

func AdjustAggregationPiecesQueued(ctx context.Context, delta int64) {
	AggregationPiecesQueued.Add(ctx, delta)
}

// Ethereum helpers

func RecordEthTransaction(ctx context.Context, txType, status string) {
	opts := metric.WithAttributes(
		attribute.String("transaction.type", txType),
		attribute.String("transaction.status", status),
	)

	EthTransactionsTotal.Add(ctx, 1, opts)
}

func RecordEthGas(ctx context.Context, gasUsed float64) {
	EthTransactionGasUsed.Record(ctx, gasUsed)
}

func RecordEthConfirmationTime(ctx context.Context, duration time.Duration) {
	EthTransactionConfirmationTime.Record(ctx, duration.Seconds())
}

// SetupMetrics sets up OpenTelemetry metrics and the Prometheus exporter.
// If setup fails, the process logs and exits.
func SetupMetrics() *prometheus.Exporter {
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
		"http.server.duration",
		metric.WithDescription("Duration of HTTP requests in seconds, by endpoint, method, and status"),
	)
	if err != nil {
		log.Fatal(err)
	}

	HTTPRequestsTotal, err = meter.Int64Counter(
		"http.server.requests",
		metric.WithDescription("Total number of HTTP requests, by endpoint, method, and status"),
	)
	if err != nil {
		log.Fatal(err)
	}

	HTTPRequestSize, err = meter.Float64Histogram(
		"http.server.request.size",
		metric.WithDescription("Size of HTTP request bodies in bytes"),
	)
	if err != nil {
		log.Fatal(err)
	}

	HTTPResponseSize, err = meter.Float64Histogram(
		"http.server.response.size",
		metric.WithDescription("Size of HTTP response bodies in bytes"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Task metrics
	TaskExecutionDuration, err = meter.Float64Histogram(
		"task.execution.duration",
		metric.WithDescription("Task execution time in seconds by task_name and status"),
	)
	if err != nil {
		log.Fatal(err)
	}

	TasksTotal, err = meter.Int64Counter(
		"task.execution",
		metric.WithDescription("Total number of tasks executed by task_name and status"),
	)
	if err != nil {
		log.Fatal(err)
	}

	TaskQueueDepth, err = meter.Int64UpDownCounter(
		"task.queue.depth",
		metric.WithDescription("Current task queue depth by queue_name"),
	)
	if err != nil {
		log.Fatal(err)
	}

	TaskRetriesTotal, err = meter.Int64Counter(
		"task.retries",
		metric.WithDescription("Total number of task retries attempted"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// PDP metrics
	ProofsSubmitted, err = meter.Int64Counter(
		"pdp.proofs.submitted",
		metric.WithDescription("Total successful proof submissions"),
	)
	if err != nil {
		log.Fatal(err)
	}

	ProofsFailed, err = meter.Int64Counter(
		"pdp.proofs.failed",
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
		"pdp.proofs.deadline",
		metric.WithDescription("Time until next proof deadline in seconds"),
	)
	if err != nil {
		log.Fatal(err)
	}

	ChallengeWindowDuration, err = meter.Float64Gauge(
		"pdp.challenge.window.duration",
		metric.WithDescription("Challenge window duration in seconds"),
	)
	if err != nil {
		log.Fatal(err)
	}

	RootsTotal, err = meter.Int64Gauge(
		"pdp.roots",
		metric.WithDescription("Total number of roots across all proof sets"),
	)
	if err != nil {
		log.Fatal(err)
	}

	PDPDataSize, err = meter.Int64Gauge(
		"pdp.data.size",
		metric.WithDescription("Total data size being proven in bytes"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Storage metrics
	StorageUsed, err = meter.Int64Gauge(
		"storage.used",
		metric.WithDescription("Total storage used by type (blob, stash) in bytes"),
	)
	if err != nil {
		log.Fatal(err)
	}

	PiecesStored, err = meter.Int64Gauge(
		"storage.pieces",
		metric.WithDescription("Total number of pieces stored"),
	)
	if err != nil {
		log.Fatal(err)
	}

	StashFilesCount, err = meter.Int64Gauge(
		"storage.stash.files",
		metric.WithDescription("Total number of stash files stored"),
	)
	if err != nil {
		log.Fatal(err)
	}

	StorageOperationsTotal, err = meter.Int64Counter(
		"storage.operations",
		metric.WithDescription("Total storage operations by type and status"),
	)
	if err != nil {
		log.Fatal(err)
	}

	StorageOperationDuration, err = meter.Float64Histogram(
		"storage.operation.duration",
		metric.WithDescription("Latency of storage operations in seconds"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Aggregation metrics
	AggregatesCreated, err = meter.Int64Counter(
		"aggregation.created",
		metric.WithDescription("Total number of aggregates created"),
	)
	if err != nil {
		log.Fatal(err)
	}

	AggregateSize, err = meter.Int64Histogram(
		"aggregation.size",
		metric.WithDescription("Size distribution of aggregates in bytes"),
	)
	if err != nil {
		log.Fatal(err)
	}

	AggregationDuration, err = meter.Float64Histogram(
		"aggregation.duration",
		metric.WithDescription("Time taken to create aggregates in seconds"),
	)
	if err != nil {
		log.Fatal(err)
	}

	AggregationPiecesQueued, err = meter.Int64UpDownCounter(
		"aggregation.pieces.queued",
		metric.WithDescription("Number of pieces waiting for aggregation"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Ethereum metrics
	EthTransactionsTotal, err = meter.Int64Counter(
		"eth.transactions",
		metric.WithDescription("Total Ethereum transactions by type and status"),
	)
	if err != nil {
		log.Fatal(err)
	}

	EthTransactionGasUsed, err = meter.Float64Histogram(
		"eth.transaction.gas",
		metric.WithDescription("Gas used per Ethereum transaction"),
	)
	if err != nil {
		log.Fatal(err)
	}

	EthTransactionConfirmationTime, err = meter.Float64Histogram(
		"eth.transaction.confirmation.time",
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
