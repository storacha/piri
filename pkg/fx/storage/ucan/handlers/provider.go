package handlers

import (
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/service/storage/ucan"
)

var Module = fx.Module("storage/ucan/handlers",
	fx.Provide(
		fx.Annotate(
			ucan.BlobAllocate,
			fx.ResultTags(`group:"ucan_options"`),
		),
		fx.Annotate(
			ucan.BlobAccept,
			fx.ResultTags(`group:"ucan_options"`),
		),
		fx.Annotate(
			ucan.PDPInfo,
			fx.ResultTags(`group:"ucan_options"`),
		),
		fx.Annotate(
			ucan.ReplicaAllocate,
			fx.ResultTags(`group:"ucan_options"`),
		),
	),
)
