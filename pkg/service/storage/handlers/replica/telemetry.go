package replica

import (
	"time"

	"github.com/storacha/piri/pkg/telemetry"
)

var (
	transferFailureCounter *telemetry.Counter
	transferDurationTimer  *telemetry.Timer
)

func init() {
	tel := telemetry.Global()

	newCounter := func(name, desc string) *telemetry.Counter {
		counter, err := tel.NewCounter(telemetry.CounterConfig{
			Name:        name,
			Description: desc,
		})
		if err != nil {
			log.Warnw("failed to init telemetry counter", "name", name, "error", err)
			return nil
		}
		return counter
	}

	newTimer := func(name, desc string, bounds []float64) *telemetry.Timer {
		timer, err := tel.NewTimer(telemetry.TimerConfig{
			Name:        name,
			Description: desc,
			Unit:        "ms",
			Boundaries:  bounds,
		})
		if err != nil {
			log.Warnw("failed to init telemetry timer", "name", name, "error", err)
			return nil
		}
		return timer
	}

	transferFailureCounter = newCounter("replica_transfer_failure", "records failures during replica transfer operations grouped by sink")
	transferDurationTimer = newTimer("replica_transfer_duration", "duration of replica transfer operations grouped by sink", telemetry.DurationMillis(
		10*time.Millisecond,
		100*time.Millisecond,
		time.Second,
		3*time.Second,
		5*time.Second,
		10*time.Second,
		30*time.Second,
		time.Minute,
		2*time.Minute,
		5*time.Minute,
		10*time.Minute,
		15*time.Minute,
		20*time.Minute,
		30*time.Minute,
	))
}
