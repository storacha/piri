package replicator

import (
	ucanserver "github.com/storacha/go-ucanto/server"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/services/replicator/ucan"
	"github.com/storacha/piri/pkg/services/types"
)

// ServiceModule provides the replicator service with lifecycle management
var ServiceModule = fx.Module("replicator",
	// Provide the service
	// The concrete *Service type also implements the types.Replicator interface
	fx.Provide(
		fx.Annotate(
			NewService,
			fx.As(new(types.Replicator)),
		),
	),
)

var UCANModule = fx.Module("replicator-http",
	// Provide individual handlers
	fx.Provide(
		ucan.NewAllocate,
	),
	// Provide server options with tags
	fx.Provide(
		fx.Annotate(
			func(h *ucan.Allocate) ucanserver.Option { return h.Option() },
			fx.ResultTags(`group:"ucan-options"`),
		),
	),
)
