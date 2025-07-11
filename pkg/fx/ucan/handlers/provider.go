package handlers

import (
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"
)

var log = logging.Logger("ucan/handlers")

var Module = fx.Module("ucan/handlers",
	fx.Provide(
		fx.Annotate(
			BlobAllocateHandler,
			fx.ResultTags(`group:"ucan_options"`),
		),
		fx.Annotate(
			BlobAcceptHandler,
			fx.ResultTags(`group:"ucan_options"`),
		),
		fx.Annotate(
			PDPInfoHandler,
			fx.ResultTags(`group:"ucan_options"`),
		),
		fx.Annotate(
			ReplicaAllocateHandler,
			fx.ResultTags(`group:"ucan_options"`),
		),
	),
)
