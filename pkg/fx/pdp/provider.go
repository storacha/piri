package pdp

import (
	"fmt"

	"github.com/storacha/filecoin-services/go/eip712"
	"github.com/storacha/piri/pkg/pdp/comper"
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
	"github.com/storacha/piri/pkg/pdp/pieces"
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
			fx.As(new(types.PieceReaderAPI)),
			fx.As(new(types.PieceWriterAPI)),
			fx.As(new(types.PieceResolverAPI)),
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
			ProvidePieceResolver,
			fx.As(new(pieces.Resolver)),
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
	comper comper.Calculator
	api    types.API
}

func (s *TODO_PDP_Impl) API() types.PieceAPI {
	return s.api
}

func (s *TODO_PDP_Impl) CommpCalculator() comper.Calculator {
	return s.comper
}

var _ pdp.PDP = (*TODO_PDP_Impl)(nil)

func ProvideTODOPDPImplInterface(service types.API, comper comper.Calculator, cfg app.AppConfig) (*TODO_PDP_Impl, error) {
	return &TODO_PDP_Impl{
		comper: comper,
		api:    service,
	}, nil
}

type Params struct {
	fx.In

	ServerConfig     app.ServerConfig
	DB               *gorm.DB `name:"engine_db"`
	Config           app.PDPServiceConfig
	Store            blobstore.PDPStore
	Stash            stashstore.Stash
	Sender           ethereum.Sender
	PieceResolver    pieces.Resolver
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
		params.ServerConfig.PublicURL,
		params.DB,
		params.Config.OwnerAddress,
		params.Store,
		params.PieceResolver,
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

type PieceResolverParams struct {
	fx.In

	DB    *gorm.DB `name:"engine_db"`
	Store blobstore.PDPStore
}

func ProvidePieceResolver(params PieceResolverParams) pieces.Resolver {
	return pieces.NewStoreResolver(params.DB, params.Store)
}

func ProviderSigningService(cfg app.SigningServiceConfig) (signer.SigningService, error) {
	if cfg.Endpoint != nil {
		return signerclient.New(cfg.Endpoint.String()), nil
	} else if cfg.PrivateKey != nil {
		s := signingservice.NewSigner(
			cfg.PrivateKey,
			smartcontracts.ChainID,
			smartcontracts.Addresses().Service,
		)
		return signerimpl.New(s), nil
	}

	return nil, fmt.Errorf("no signer configured")
}
