package manager

import (
	"go.uber.org/fx"
)

var Module = fx.Module("aggregation/manager",
	fx.Provide(
		NewConfigProvider,
		NewManager,
		NewSubmissionWorkspace,
		NewAddRootsTaskHandler,
		NewPieceAccepter,
		NewQueue,
	),
)
