package pdp

import (
	"go.uber.org/fx"

	ucanserver "github.com/storacha/go-ucanto/server"

	"github.com/storacha/piri/pkg/services/pdp/ucan"
)

// UCANModule provides pdp-related UCAN handlers
var UCANModule = fx.Module("pdp-ucan",
	// Provide individual handlers
	fx.Provide(
		ucan.NewInfo,
	),
	// Provide server options with tags
	fx.Provide(
		fx.Annotate(
			func(h *ucan.Info) ucanserver.Option { return h.Option() },
			fx.ResultTags(`group:"ucan-options"`),
		),
	),
)
