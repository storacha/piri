package egresstracking

import (
	"go.uber.org/fx"

	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/piri/pkg/config/app"
	echofx "github.com/storacha/piri/pkg/fx/echo"
	"github.com/storacha/piri/pkg/service/egresstracking"
	"github.com/storacha/piri/pkg/store/egressbatchstore"
)

var Module = fx.Module("egresstracking",
	fx.Provide(
		NewService,
		fx.Annotate(
			egresstracking.NewServer,
			fx.As(new(echofx.RouteRegistrar)),
			fx.ResultTags(`group:"route_registrar"`),
		),
	),
)

func NewService(id principal.Signer, store egressbatchstore.EgressBatchStore, cfg app.AppConfig) *egresstracking.EgressTrackingService {
	batchEndpoint := cfg.Server.PublicURL.JoinPath(egresstracking.ReceiptsPath + "/{cid}")
	egressTrackerConn := cfg.UCANService.Services.EgressTracker.Connection
	egressTrackerProofs := cfg.UCANService.Services.EgressTracker.Proofs

	return egresstracking.New(
		id,
		egressTrackerConn,
		egressTrackerProofs,
		batchEndpoint,
		store,
	)
}
