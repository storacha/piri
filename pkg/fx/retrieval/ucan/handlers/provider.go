package handlers

import (
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/service/retrieval/ucan"
)

var Module = fx.Module("retrieval/ucan/handlers",
	fx.Provide(
		fx.Annotate(
			ucan.SpaceContentRetrieve,
			fx.ResultTags(`group:"ucan_retrieval_options"`),
		),
	),
)
