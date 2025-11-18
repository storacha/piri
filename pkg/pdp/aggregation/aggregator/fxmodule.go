package aggregator

import (
	"go.uber.org/fx"
)

var Module = fx.Module("aggregation/aggregator",
	fx.Provide(
		New,
		NewQueue,
		NewHandler,
	),
)
