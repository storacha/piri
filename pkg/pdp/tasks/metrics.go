package tasks

import (
	"sync"

	"github.com/storacha/piri/pkg/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

var (
	ProveTaskDuration *telemetry.Timer
	ProveAtEpoch      *telemetry.Gauge

	metricsOnce sync.Once
)

// InitMetrics initializes all PDP task metrics lazily
func InitMetrics() {
	metricsOnce.Do(func() {
		tel := telemetry.Global()

		var err error

		// Timer for proof generation duration
		ProveTaskDuration, err = tel.NewTimer(telemetry.TimerConfig{
			Name:        "pdp_prove_task_duration",
			Description: "Duration of proof generation in milliseconds",
			Unit:        "ms",
			Boundaries:  telemetry.LatencyBoundaries,
		})
		if err != nil {
			log.Warnw("failed to initialize pdp_prove_task_duration metric", "error", err)
		}
		// Gauge for next prove at epoch
		ProveAtEpoch, err = tel.NewGauge(telemetry.GaugeConfig{
			Name:        "pdp_next_prove_epoch",
			Description: "The next epoch a proof may be submitted during",
			Unit:        "epoch",
		})
		if err != nil {
			log.Warnw("failed to initialize pdp_next_prove_epoch metric", "error", err)
		}
	})
}

// Attribute helpers for consistent labeling
func ProofSetIDAttr(id int64) attribute.KeyValue {
	return telemetry.Int64Attr("proof_set_id", id)
}
