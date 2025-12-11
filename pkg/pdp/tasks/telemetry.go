package tasks

import (
	"github.com/storacha/piri/pkg/telemetry"
)

var (
	MessageEstimateGasFailureCounter *telemetry.Counter
	MessageSendFailureCounter        *telemetry.Counter
	PDPProveFailureCounter           *telemetry.Counter
	PDPNextFailureCounter            *telemetry.Counter
	PDPAddPieceFailureCounter        *telemetry.Counter
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

	MessageEstimateGasFailureCounter = newCounter("pdp_message_estimate_gas_failure",
		"records failure to estimate gas for sending messages; similar to a send failure")
	MessageSendFailureCounter = newCounter("pdp_message_send_failure", "records failure to send a message")
	PDPNextFailureCounter = newCounter("pdp_next_failure", "records failure in next pdp task")
	PDPProveFailureCounter = newCounter("pdp_prove_failure", "records failure to submit a pdp proof")
	PDPAddPieceFailureCounter = newCounter("pdp_add_piece_failure", "records failure to add a pdp piece")

}
