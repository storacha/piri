package smartcontracts

import (
	"go.uber.org/fx"
)

var Module = fx.Module("smartcontracts",
	fx.Provide(
		NewRegistry,
		NewServiceView,
		NewVerifierContract,
	),
)
