package replica

import (
	"context"
	"time"

	"github.com/storacha/piri/pkg/telemetry"
)

type Metrics struct {
	failureCounter *telemetry.Counter
	durationTimer  *telemetry.Timer
}

func NewMetrics(tel *telemetry.Telemetry) *Metrics {
	if tel == nil {
		tel = telemetry.Global()
	}

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

	return &Metrics{
		failureCounter: newCounter("replica_transfer_failure", "records failures during replica transfer operations grouped by sink"),
		durationTimer: newTimer("replica_transfer_duration", "duration of replica transfer operations grouped by sink", telemetry.DurationMillis(
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
		)),
	}
}

func (m *Metrics) recordFailure(ctx context.Context, sink string) {
	if m == nil || m.failureCounter == nil {
		return
	}
	m.failureCounter.Inc(ctx, telemetry.StringAttr("sink", sink))
}

func (m *Metrics) startDuration(ctx context.Context, sink string) *telemetry.TimedContext {
	if m == nil || m.durationTimer == nil {
		return nil
	}
	return m.durationTimer.WithAttributes(telemetry.StringAttr("sink", sink)).Start(ctx)
}
