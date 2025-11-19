package pdp

import (
	"fmt"

	"github.com/storacha/filecoin-services/go/eip712"
	"go.uber.org/fx"
	"gorm.io/gorm"

	signerclient "github.com/storacha/piri-signing-service/pkg/client"
	signerimpl "github.com/storacha/piri-signing-service/pkg/inprocess"
	signingservice "github.com/storacha/piri-signing-service/pkg/signer"
	signer "github.com/storacha/piri-signing-service/pkg/types"

	"github.com/storacha/piri/pkg/config/app"
	echofx "github.com/storacha/piri/pkg/fx/echo"
	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/pdp/aggregator"
	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/ethereum"
	"github.com/storacha/piri/pkg/pdp/httpapi/server"
	"github.com/storacha/piri/pkg/pdp/pieceadder"
	"github.com/storacha/piri/pkg/pdp/piecefinder"
	"github.com/storacha/piri/pkg/pdp/piecereader"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/store/stashstore"
)

var Module = fx.Module("pdp-service",
	fx.Provide(
		eip712.NewExtraDataEncoder,
		ProviderSigningService,
		fx.Annotate(
			ProvidePDPService,
			fx.As(fx.Self()),      // provide service as concrete type
			fx.As(new(types.API)), // also provide the server as the interface(s) it implements
			fx.As(new(types.ProofSetAPI)),
			fx.As(new(types.PieceAPI)),
		),
		fx.Annotate(
			ProvideProofSetIDProvider,
			fx.As(new(aggregator.ProofSetIDProvider)),
		),

		fx.Annotate(
			ProvideTODOPDPImplInterface,
			fx.As(new(pdp.PDP)),
		),
		fx.Annotate(
			server.NewPDPHandler,
			fx.As(new(echofx.RouteRegistrar)),
			fx.ResultTags(`group:"route_registrar"`),
		),
	),
)

// TODO(forrest): this interface and it's impls need to be removed, renamed, or merged with the blob interface
type TODO_PDP_Impl struct {
	aggregator  aggregator.Aggregator
	pieceFinder piecefinder.PieceFinder
	pieceAdder  pieceadder.PieceAdder
	pieceReader piecereader.PieceReader
}

func (s *TODO_PDP_Impl) PieceReader() piecereader.PieceReader {
	return s.pieceReader
}

func (s *TODO_PDP_Impl) PieceAdder() pieceadder.PieceAdder {
	return s.pieceAdder
}

func (s *TODO_PDP_Impl) PieceFinder() piecefinder.PieceFinder {
	return s.pieceFinder
}

func (s *TODO_PDP_Impl) Aggregator() aggregator.Aggregator {
	return s.aggregator
}

var _ pdp.PDP = (*TODO_PDP_Impl)(nil)

func ProvideTODOPDPImplInterface(service types.API, agg aggregator.Aggregator, cfg app.AppConfig) (*TODO_PDP_Impl, error) {
	finder := piecefinder.New(service, &cfg.Server.PublicURL)
	adder := pieceadder.New(service, &cfg.Server.PublicURL)
	reader := piecereader.New(service, &cfg.Server.PublicURL)
	return &TODO_PDP_Impl{
		aggregator:  agg,
		pieceFinder: finder,
		pieceAdder:  adder,
		pieceReader: reader,
	}, nil
}

type Params struct {
	fx.In

	DB               *gorm.DB `name:"engine_db"`
	Config           app.PDPServiceConfig
	Store            blobstore.PDPStore
	Stash            stashstore.Stash
	Sender           ethereum.Sender
	Engine           *scheduler.TaskEngine
	ChainScheduler   *chainsched.Scheduler
	ChainClient      service.ChainClient
	ContractBackend  service.EthClient
	SigningService   signer.SigningService
	ExtraDataEncoder *eip712.ExtraDataEncoder
	Verifier         smartcontracts.Verifier
	Service          smartcontracts.Service
	Registry         smartcontracts.Registry
}

func ProvidePDPService(params Params) (*service.PDPService, error) {
	return service.New(
		params.DB,
		params.Config,
		params.Store,
		params.Stash,
		params.Sender,
		params.Engine,
		params.ChainScheduler,
		params.ChainClient,
		params.ContractBackend,
		params.SigningService,
		params.ExtraDataEncoder,
		params.Verifier,
		params.Service,
		params.Registry,
	)
}

func ProvideProofSetIDProvider(cfg app.UCANServiceConfig) (*aggregator.ConfiguredProofSetProvider, error) {
	return &aggregator.ConfiguredProofSetProvider{ID: cfg.ProofSetID}, nil
}

func ProviderSigningService(cfg app.PDPServiceConfig) (signer.SigningService, error) {
	if cfg.SigningServiceConfig.Endpoint != nil {
		return signerclient.New(cfg.SigningServiceConfig.Endpoint.String()), nil
	} else if cfg.SigningServiceConfig.PrivateKey != nil {
		s := signingservice.NewSigner(
			cfg.SigningServiceConfig.PrivateKey,
			cfg.ChainID,
			cfg.Contracts.Service,
		)
		return signerimpl.New(s), nil
	}

	return nil, fmt.Errorf("no signer configured")
}
