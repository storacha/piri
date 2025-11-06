package piece

import (
	"go.uber.org/fx"
)

var Module = fx.Module("pdp/piece",
	fx.Provide(
		NewStoreResolver,

		NewStoreReader,

		NewComper,
		NewCommpQueue,
		NewComperTaskHandler,
	),
)
