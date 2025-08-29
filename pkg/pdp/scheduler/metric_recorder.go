package scheduler

/**
import (
	"context"
	"time"

	"github.com/storacha/piri/pkg/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func recordTask(ctx context.Context, taskID TaskID, start time.Time, isSuccessful bool) {
	status := "success"
	if !isSuccessful {
		status = "failed"
	}

	taskName := ""

	opts := metric.WithAttributes(
		attribute.String("task.name", taskName),
		attribute.String("task.status", status),
	)

	telemetry.TaskExecutionDuration.Record(context.Background(), time.Since(start).Seconds(), opts)
	telemetry.TasksTotal.Add(context.Background(), 1, opts)
}

func recordTaskRetry(ctx context.Context, taskName string) {
	opts := metric.WithAttributes(
		attribute.String("task.name", taskName),
	)

	telemetry.TaskRetriesTotal.Add(context.Background(), 1, opts)
}

// ObserveQueueDepth reports current queue depth for a named queue.
func observeQueueDepth(ctx context.Context, queueName string, depth int64) {

	opts := metric.WithAttributes(
		attribute.String("queue.name", queueName),
	)

	telemetry.TaskQueueDepth.Add(context.Background(), depth, opts)
}
**/
