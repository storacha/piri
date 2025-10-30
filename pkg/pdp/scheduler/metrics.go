package scheduler

import (
	"sync"

	"github.com/storacha/piri/pkg/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

var (
	TaskDuration *telemetry.Timer
	TaskSuccess  *telemetry.Counter
	TaskFailure  *telemetry.Counter

	metricsOnce sync.Once
)

// InitMetrics initializes all PDP task metrics lazily
func InitMetrics() {
	metricsOnce.Do(func() {
		tel := telemetry.Global()

		var err error

		// Timer for proof generation duration
		TaskDuration, err = tel.NewTimer(telemetry.TimerConfig{
			Name:        "engine_task_duration",
			Description: "Duration of task in milliseconds",
			Unit:        "ms",
			Boundaries:  telemetry.LatencyBoundaries,
		})
		if err != nil {
			log.Warnw("failed to initialize engine_task_duration metric", "error", err)
		}
		TaskSuccess, err = tel.NewCounter(telemetry.CounterConfig{
			Name:        "engine_task_success",
			Description: "Number of successful tasks",
			Unit:        "count",
		})
		if err != nil {
			log.Warnw("failed to initialize engine_task_success metric", "error", err)
		}
		TaskFailure, err = tel.NewCounter(telemetry.CounterConfig{
			Name:        "engine_task_failure",
			Description: "Number of failed tasks",
			Unit:        "count",
		})
		if err != nil {
			log.Warnw("failed to initialize engine_task_failure metric", "error", err)
		}
	})
}

// Attribute helpers for consistent labeling
func TaskType(name string) attribute.KeyValue {
	return telemetry.StringAttr("task_name", name)
}
