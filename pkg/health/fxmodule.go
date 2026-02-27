package health

import (
	"go.uber.org/fx"

	echofx "github.com/storacha/piri/pkg/fx/echo"
)

// CheckerParams defines the parameters for NewChecker with optional ServerMode
type CheckerParams struct {
	fx.In

	Mode ServerMode `optional:"true"`
}

// NewCheckerFromParams creates a new Checker from fx parameters
func NewCheckerFromParams(params CheckerParams) *Checker {
	mode := params.Mode
	if mode == "" {
		mode = ModeFull // Default to full mode for backwards compatibility
	}
	return NewChecker(mode)
}

// Module provides health check functionality
var Module = fx.Module("health",
	fx.Provide(
		NewCheckerFromParams,
		fx.Annotate(
			NewHandler,
			fx.As(new(echofx.RouteRegistrar)),
			fx.ResultTags(`group:"route_registrar"`),
		),
	),
)
