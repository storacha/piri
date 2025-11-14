package aggregation

import (
	"github.com/storacha/piri/pkg/pdp/aggregation/aggregator"
	"github.com/storacha/piri/pkg/pdp/aggregation/commp"
	"github.com/storacha/piri/pkg/pdp/aggregation/manager"
	"github.com/storacha/piri/pkg/pdp/aggregation/types"
	"go.uber.org/fx"
)

var Module = fx.Module("aggregation",
	commp.Module,
	aggregator.Module,
	manager.Module,
	fx.Provide(
		types.NewStore,
	),
)
