package aggregator

import (
	"go.uber.org/fx"
)

var Module = fx.Module("aggregator",
	// Provide the queue implementation shared by all below
	fx.Provide(
		NewQueues,
	),

	// calculate CommP hashes of blobs
	fx.Provide(
		NewQueuingCommpCalculator,
		NewComperTaskHandler,
	),

	// aggregate CommP-hashed data into aggregates
	fx.Provide(
		NewAggregator,
		NewAggregatorHandler,
		NewAggregatorStore,
		NewInProgressWorkspace,
	),

	// submit aggregations of aggregates to PDP
	fx.Provide(
		NewManager,
		NewSubmissionWorkspace,
		NewAddRootsTaskHandler,
	),

)
