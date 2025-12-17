package replica

import (
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/storacha/piri/lib/telemetry"
)

var (
	tracer = otel.Tracer("github.com/storacha/piri/pkg/service/storage/handlers/replica")
)

var replicaDurationBounds = []float64{
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

type Metrics struct {
	durationTimer *telemetry.Timer
}

func NewMetrics() (*Metrics, error) {
	meter := otel.GetMeterProvider().Meter("github.com/storacha/piri/pkg/service/storage/handlers/replica")
	durationTimer, err := telemetry.NewTimer(
		meter,
		"transfer_duration",
		"durating of replica transfer operation",
		replicaDurationBounds,
	)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		durationTimer: durationTimer,
	}, nil
}

func (m *Metrics) startDuration(source string) *telemetry.StopWatch {
	if m == nil || m.durationTimer == nil {
		return nil
	}
	return m.durationTimer.Start(attribute.String("source", source))
}
