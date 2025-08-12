package pdp

import (
	"go.uber.org/fx"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/config/app"
	echofx "github.com/storacha/piri/pkg/fx/echo"
	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/pdp/aggregator"
	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/ethereum"
	"github.com/storacha/piri/pkg/pdp/httpapi/server"
	"github.com/storacha/piri/pkg/pdp/pieceadder"
	"github.com/storacha/piri/pkg/pdp/piecefinder"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/store/stashstore"
)

var Module = fx.Module("pdp-service",
	fx.Provide(
		fx.Annotate(
			ProvidePDPService,
			fx.As(fx.Self()),      // provide service as concrete type
			fx.As(new(types.API)), // also provide the server as the interface(s) it implements
			fx.As(new(types.ProofSetAPI)),
			fx.As(new(types.PieceAPI)),
		),
		fx.Annotate(
			ProvideProofSetIDProvider,
			fx.As(new(types.ProofSetIDProvider)),
		),

		fx.Annotate(
			ProvidePoorlyNamedPDPInterface,
			fx.As(new(pdp.PDP)),
		),
		fx.Annotate(
			server.NewPDPServer,
			fx.As(new(echofx.RouteRegistrar)),
			fx.ResultTags(`group:"route_registrar"`),
		),
	),
)

type Shit struct {
	aggregator  aggregator.Aggregator
	pieceFinder piecefinder.PieceFinder
	pieceAdder  pieceadder.PieceAdder
}

func (s *Shit) PieceAdder() pieceadder.PieceAdder {
	return s.pieceAdder
}

func (s *Shit) PieceFinder() piecefinder.PieceFinder {
	return s.pieceFinder
}

func (s *Shit) Aggregator() aggregator.Aggregator {
	return s.aggregator
}

func ProvidePoorlyNamedPDPInterface(service types.API, agg aggregator.Aggregator, cfg app.AppConfig) (*Shit, error) {
	finder := piecefinder.New(service, &cfg.Server.PublicURL)
	adder := pieceadder.New(service, &cfg.Server.PublicURL)
	return &Shit{
		aggregator:  agg,
		pieceFinder: finder,
		pieceAdder:  adder,
	}, nil
}

type Params struct {
	fx.In

	DB             *gorm.DB `name:"engine_db"`
	Config         app.PDPServiceConfig
	Store          blobstore.PDPStore
	Stash          stashstore.Stash
	Sender         ethereum.Sender
	Engine         *scheduler.TaskEngine
	ChainScheduler *chainsched.Scheduler
}

func ProvidePDPService(params Params) (*service.PDPService, error) {
	return service.New(
		params.DB,
		params.Config.Local.OwnerAddress,
		params.Store,
		params.Stash,
		params.Sender,
		params.Engine,
		params.ChainScheduler,
	)
}

func ProvideProofSetIDProvider(cfg app.PDPServiceConfig) (*aggregator.ConfiguredProofSetIDProvider, error) {
	return &aggregator.ConfiguredProofSetIDProvider{ID: cfg.Local.ProofSetID}, nil
}
