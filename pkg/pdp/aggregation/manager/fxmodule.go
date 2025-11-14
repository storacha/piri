package manager

import (
	"go.uber.org/fx"
)

var Module = fx.Module("aggregation/manager",
	fx.Provide(
		NewManager,
		NewSubmissionWorkspace,
		NewAddRootsTaskHandler,
		NewPieceAccepter,
		NewQueue,
	),
)
