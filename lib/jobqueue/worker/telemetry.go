package worker

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"go.opentelemetry.io/otel/attribute"

	"github.com/storacha/piri/pkg/telemetry"
)

var telemetryLog = logging.Logger("jobqueue/telemetry")

// jobDurationBounds are in milliseconds, covering 5ms up to 30 minutes.
var jobDurationBounds = telemetry.DurationMillis(
	5*time.Millisecond,
	10*time.Millisecond,
	25*time.Millisecond,
	50*time.Millisecond,
	75*time.Millisecond,
	100*time.Millisecond,
	250*time.Millisecond,
	500*time.Millisecond,
	750*time.Millisecond,
	time.Second,
	2500*time.Millisecond,
	5*time.Second,
	7500*time.Millisecond,
	10*time.Second,
	30*time.Second,
	time.Minute,
	2*time.Minute,
	5*time.Minute,
	10*time.Minute,
	15*time.Minute,
	20*time.Minute,
	30*time.Minute,
)

type metricsKey struct {
	queue string
	job   string
}

type metricsRecorder struct {
	activeJobsGauge   *telemetry.Gauge
	queuedJobsGauge   *telemetry.Gauge
	failedJobsCounter *telemetry.Counter
	jobDurationTimer  *telemetry.Timer

	queuedGaugeCounts sync.Map // map[metricsKey]*atomic.Int64
	activeGaugeCounts sync.Map // map[metricsKey]*atomic.Int64
}

func newMetrics(tel *telemetry.Telemetry) *metricsRecorder {
	if tel == nil {
		tel = telemetry.Global()
	}

	newGauge := func(name, desc string) *telemetry.Gauge {
		gauge, err := tel.NewGauge(telemetry.GaugeConfig{
			Name:        name,
			Description: desc,
			Unit:        "jobs",
		})
		if err != nil {
			telemetryLog.Warnw("failed to init telemetry gauge", "name", name, "error", err)
			return nil
		}
		return gauge
	}
	newCounter := func(name, desc string) *telemetry.Counter {
		counter, err := tel.NewCounter(telemetry.CounterConfig{
			Name:        name,
			Description: desc,
		})
		if err != nil {
			telemetryLog.Warnw("failed to init telemetry counter", "name", name, "error", err)
			return nil
		}
		return counter
	}

	timer, err := tel.NewTimer(telemetry.TimerConfig{
		Name:        "jobqueue_job_duration",
		Description: "time spent running a job until success or failure",
		Unit:        "ms",
		Boundaries:  jobDurationBounds,
	})
	if err != nil {
		telemetryLog.Warnw("failed to init telemetry timer", "name", "jobqueue_job_duration", "error", err)
	}

	return &metricsRecorder{
		activeJobsGauge:   newGauge("jobqueue_active_jobs", "number of jobs currently running"),
		queuedJobsGauge:   newGauge("jobqueue_queued_jobs", "number of jobs waiting to be processed"),
		failedJobsCounter: newCounter("jobqueue_failed_jobs", "records jobs that failed permanently or exhausted retries"),
		jobDurationTimer:  timer,
	}
}

func (m *metricsRecorder) recordQueuedDelta(ctx context.Context, queueName, jobName string, delta int64) {
	if m == nil {
		return
	}
	recordGaugeDelta(ctx, m.queuedJobsGauge, &m.queuedGaugeCounts, queueName, jobName, delta)
}

func (m *metricsRecorder) recordActiveDelta(ctx context.Context, queueName, jobName string, delta int64) {
	if m == nil {
		return
	}
	recordGaugeDelta(ctx, m.activeJobsGauge, &m.activeGaugeCounts, queueName, jobName, delta)
}

func (m *metricsRecorder) recordJobFailure(ctx context.Context, queueName, jobName, reason string, attempt int) {
	if m == nil || m.failedJobsCounter == nil || queueName == "" || jobName == "" {
		return
	}

	attrs := []attribute.KeyValue{
		telemetry.StringAttr("queue", queueName),
		telemetry.StringAttr("job", jobName),
	}
	if reason != "" {
		attrs = append(attrs, telemetry.StringAttr("failure_reason", reason))
	}
	if attempt > 0 {
		attrs = append(attrs, telemetry.IntAttr("attempt", attempt))
	}

	m.failedJobsCounter.Inc(ctx, attrs...)
}

func (m *metricsRecorder) recordJobDuration(ctx context.Context, queueName, jobName, status string, attempt int, duration time.Duration) {
	if m == nil || m.jobDurationTimer == nil || queueName == "" || jobName == "" {
		return
	}

	attrs := []attribute.KeyValue{
		telemetry.StringAttr("queue", queueName),
		telemetry.StringAttr("job", jobName),
		telemetry.StringAttr("status", status),
	}
	if attempt > 0 {
		attrs = append(attrs, telemetry.IntAttr("attempt", attempt))
	}

	m.jobDurationTimer.Record(ctx, duration, attrs...)
}

func recordGaugeDelta(ctx context.Context, gauge *telemetry.Gauge, counts *sync.Map, queueName, jobName string, delta int64) {
	if gauge == nil || queueName == "" || jobName == "" {
		return
	}

	key := metricsKey{queue: queueName, job: jobName}
	val, _ := counts.LoadOrStore(key, &atomic.Int64{})
	current := val.(*atomic.Int64).Add(delta)
	if current < 0 {
		val.(*atomic.Int64).Store(0)
		current = 0
	}

	gauge.Record(ctx, current, telemetry.StringAttr("queue", queueName), telemetry.StringAttr("job", jobName))
}
