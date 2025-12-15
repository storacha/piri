package service

import (
	"go.opentelemetry.io/otel"

	"github.com/storacha/piri/pkg/telemetry"
)

var (
	tracer = otel.Tracer("github.com/storacha/piri/pkg/pdp/service")
)

var (
	PDPAddPieceFailureCounter *telemetry.Counter
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

	PDPAddPieceFailureCounter = newCounter("pdp_add_piece_failure", "records failure to add a pdp piece")

}
