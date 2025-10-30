package tasks

import (
	"sync"

	"github.com/storacha/piri/pkg/telemetry"
)

var (
	ProveTaskDuration *telemetry.Timer
	metricsOnce       sync.Once
)

// InitMetrics initializes all PDP task metrics lazily
func InitMetrics() {
	metricsOnce.Do(func() {
		tel := telemetry.Global()

		var err error

		// Timer for proof generation duration
		ProveTaskDuration, err = tel.NewTimer(telemetry.TimerConfig{
			Name:        "pdp_generate_proof_duration",
			Description: "Duration of proof generation in milliseconds",
			Unit:        "ms",
			Boundaries:  telemetry.LatencyBoundaries,
		})
		if err != nil {
			log.Warnw("failed to initialize pdp_prove_task_duration metric", "error", err)
		}
	})
}
