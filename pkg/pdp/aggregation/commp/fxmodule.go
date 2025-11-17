package commp

import (
	"go.uber.org/fx"
)

var Module = fx.Module("aggregation/commp",
	fx.Provide(
		NewQueue,
		NewHandler,
		NewQueuingCommpCalculator,
	),
)
