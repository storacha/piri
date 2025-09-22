package egresstracking

import (
	"go.uber.org/fx"

	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/service/egresstracking"
	"github.com/storacha/piri/pkg/store/egressbatchstore"
)

var Module = fx.Module("egresstracking",
	fx.Provide(NewService),
)

func NewService(id principal.Signer, store egressbatchstore.EgressBatchStore, cfg app.AppConfig) *egresstracking.EgressTrackingService {
	batchEndpoint := cfg.Server.PublicURL.JoinPath("/receipts/{cid}")
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
