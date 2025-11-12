package pdp

import (
	"fmt"

	"github.com/storacha/filecoin-services/go/eip712"
	"github.com/storacha/piri/pkg/pdp/piece"
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
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store/blobstore"
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
			fx.As(new(types.PieceWriterAPI)),
			fx.As(new(types.PieceCommPAPI)),
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
	commpCalc piece.Calculator
	api       types.PieceAPI
}

func (s *TODO_PDP_Impl) CommpCalculate() piece.Calculator {
	return s.commpCalc
}

func (s *TODO_PDP_Impl) API() types.PieceAPI {
	return s.api
}

var _ pdp.PDP = (*TODO_PDP_Impl)(nil)

func ProvideTODOPDPImplInterface(service types.API, commpCalc piece.Calculator, cfg app.AppConfig) (*TODO_PDP_Impl, error) {
	return &TODO_PDP_Impl{
		commpCalc: commpCalc,
		api:       service,
	}, nil
}

type Params struct {
	fx.In

	ServerConfig app.ServerConfig
	DB           *gorm.DB `name:"engine_db"`
	Config       app.PDPServiceConfig
	Store        blobstore.PDPStore
	Resolver     types.PieceResolverAPI
	Reader       types.PieceReaderAPI
	// TODO remove stash store, its unused.
	Sender           ethereum.Sender
	Engine           *scheduler.TaskEngine
	ChainScheduler   *chainsched.Scheduler
	ChainClient      service.ChainClient
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
		params.Resolver,
		params.Reader,
		params.Sender,
		params.Engine,
		params.ChainScheduler,
		params.ChainClient,
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
