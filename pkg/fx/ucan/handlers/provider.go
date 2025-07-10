package handlers

import (
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"
)

/*
// Providers
fx.Provide(
    fx.Annotate(
        NewUploadServiceConnection,
        fx.ResultTags(`group:"connections"`),
    ),
    fx.Annotate(
        NewIndexingServiceConnection,
        fx.ResultTags(`group:"connections"`),
    ),
)

// Consumer
type Params struct {
    fx.In

    // ... other fields
    Connections []client.Connection `group:"connections"`
}
*/

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
