package worker

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/storacha/piri/lib/telemetry"
)

// jobDurationBounds covering 5ms up to 30 minutes.
var jobDurationBounds = []float64{
	(5 * time.Millisecond).Seconds(),
	(10 * time.Millisecond).Seconds(),
	(100 * time.Millisecond).Seconds(),
	(time.Second).Seconds(),
	(3 * time.Second).Seconds(),
	(5 * time.Second).Seconds(),
	(10 * time.Second).Seconds(),
	(30 * time.Second).Seconds(),
	(time.Minute).Seconds(),
	(2 * time.Minute).Seconds(),
	(3 * time.Minute).Seconds(),
	(5 * time.Minute).Seconds(),
	(6 * time.Minute).Seconds(),
	(7 * time.Minute).Seconds(),
	(8 * time.Minute).Seconds(),
	(9 * time.Minute).Seconds(),
	(10 * time.Minute).Seconds(),
	(30 * time.Minute).Seconds(),
}

type metricsRecorder struct {
	activeJobs        *telemetry.UpDownCounter
	queuedJobs        *telemetry.UpDownCounter
	failedJobsCounter *telemetry.Counter
	jobDurationTimer  *telemetry.Timer
}

func newMetrics() (*metricsRecorder, error) {
	meter := otel.GetMeterProvider().Meter("lib/jobqueue/worker")
	activeJobs, err := telemetry.NewUpDownCounter(
		meter,
		"active_jobs",
		"number of jobs running",
		"1",
	)
	if err != nil {
		return nil, err
	}
	queuedJobs, err := telemetry.NewUpDownCounter(
		meter,
		"queued_jobs",
		"number of jobs queued (includes active)",
		"1",
	)
	if err != nil {
		return nil, err
	}
	failedJobs, err := telemetry.NewCounter(
		meter,
		"failed_jobs",
		"number of jobs that failed permanently",
		"1",
	)
	jobDuration, err := telemetry.NewTimer(
		meter,
		"job_duration",
		"records duration of a jobs runtime",
		jobDurationBounds,
	)
	return &metricsRecorder{
		activeJobs:        activeJobs,
		queuedJobs:        queuedJobs,
		failedJobsCounter: failedJobs,
		jobDurationTimer:  jobDuration,
	}, nil
}

func (m *metricsRecorder) recordQueuedDelta(ctx context.Context, queueName, jobName string, delta int64) {
	if m == nil || m.queuedJobs == nil {
		return
	}
	m.queuedJobs.Add(ctx, delta, attribute.String("queue", queueName), attribute.String("job", jobName))
}

func (m *metricsRecorder) recordActiveDelta(ctx context.Context, queueName, jobName string, delta int64) {
	if m == nil || m.activeJobs == nil {
		return
	}
	m.activeJobs.Add(ctx, delta, attribute.String("queue", queueName), attribute.String("job", jobName))
}

func (m *metricsRecorder) recordJobFailure(ctx context.Context, queueName, jobName, reason string, attempt int) {
	if m == nil || m.failedJobsCounter == nil || queueName == "" || jobName == "" {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("queue", queueName),
		attribute.String("job", jobName),
	}
	if reason != "" {
		attrs = append(attrs, attribute.String("failure_reason", reason))
	}
	if attempt > 0 {
		attrs = append(attrs, attribute.Int("attempt", attempt))
	}

	m.failedJobsCounter.Inc(ctx, attrs...)
}

func (m *metricsRecorder) recordJobDuration(ctx context.Context, queueName, jobName, status string, attempt int, duration time.Duration) {
	if m == nil || m.jobDurationTimer == nil || queueName == "" || jobName == "" {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("queue", queueName),
		attribute.String("job", jobName),
		attribute.String("status", status),
	}
	if attempt > 0 {
		attrs = append(attrs, attribute.Int("attempt", attempt))
	}

	m.jobDurationTimer.Record(ctx, duration, attrs...)
}
