package pdp

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
	"github.com/storacha/piri/tools/service-operator/eip712"
	"github.com/storacha/piri/tools/signing-service/client"
	"github.com/storacha/piri/tools/signing-service/inprocess"
	sstypes "github.com/storacha/piri/tools/signing-service/types"
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
	"github.com/storacha/piri/pkg/pdp/piecereader"
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

	DB              *gorm.DB `name:"engine_db"`
	Config          app.PDPServiceConfig
	Store           blobstore.PDPStore
	Stash           stashstore.Stash
	Sender          ethereum.Sender
	Engine          *scheduler.TaskEngine
	ChainScheduler  *chainsched.Scheduler
	ChainClient     service.ChainClient
	ContractClient  smartcontracts.PDP
	ContractBackend service.EthClient
}

func ProvidePDPService(params Params) (*service.PDPService, error) {
	// Initialize signing service if configured
	var signingService sstypes.SigningService
	var payerAddress common.Address
	var serviceContractAddress common.Address

	if params.Config.SigningService.Enabled {
		cfg := params.Config.SigningService
		payerAddress = cfg.PayerAddress
		serviceContractAddress = cfg.ServiceContractAddress

		// Choose between HTTP client and in-process signer
		if cfg.Endpoint != nil {
			// Use HTTP client for remote signing service
			signingService = client.New(cfg.Endpoint.String())
		} else if cfg.PrivateKey != "" {
			// Use in-process signer with provided private key
			privateKeyHex := strings.TrimPrefix(cfg.PrivateKey, "0x")
			privateKeyBytes, err := hex.DecodeString(privateKeyHex)
			if err != nil {
				return nil, fmt.Errorf("failed to decode private key: %w", err)
			}

			privateKey, err := crypto.ToECDSA(privateKeyBytes)
			if err != nil {
				return nil, fmt.Errorf("failed to parse private key: %w", err)
			}

			// Create in-process signer
			fmt.Printf("DEBUG: Creating signer with:\n")
			fmt.Printf("  ChainID: %d\n", cfg.ChainID)
			fmt.Printf("  ServiceContractAddress (verifying contract): %s\n", serviceContractAddress.Hex())
			fmt.Printf("  PayerAddress: %s\n", payerAddress.Hex())
			fmt.Printf("  OwnerAddress (service provider/payee): %s\n", params.Config.OwnerAddress.Hex())

			signer := eip712.NewSigner(
				privateKey,
				big.NewInt(cfg.ChainID),
				serviceContractAddress,
			)

			fmt.Printf("  Signer address (from private key): %s\n", signer.GetAddress().Hex())
			signingService = inprocess.New(signer)
		} else {
			return nil, fmt.Errorf("signing service enabled but no endpoint or private key configured")
		}
	}

	return service.New(
		params.DB,
		params.Config.OwnerAddress,
		params.Store,
		params.Stash,
		params.Sender,
		params.Engine,
		params.ChainScheduler,
		params.ChainClient,
		params.ContractClient,
		params.ContractBackend,
		signingService,
		payerAddress,
		serviceContractAddress,
	)
}

func ProvideProofSetIDProvider(cfg app.UCANServiceConfig) (*aggregator.ConfiguredProofSetProvider, error) {
	return &aggregator.ConfiguredProofSetProvider{ID: cfg.ProofSetID}, nil
}
