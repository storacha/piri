package pdp

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/ethereum/go-ethereum/common"
	"go.uber.org/fx"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/pdp/aggregator"
	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/ethereum"
	"github.com/storacha/piri/pkg/pdp/pieceadder"
	"github.com/storacha/piri/pkg/pdp/piecefinder"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service"
	"github.com/storacha/piri/pkg/pdp/store"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store/blobstore"
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
			ProvideProofSetProvider,
			fx.As(new(types.ProofSetProvider)),
		),
		fx.Annotate(
			ProvidePoorlyNamedPDPInterface,
			fx.As(new(pdp.PDP)),
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
	finder := piecefinder.New(service, cfg.Server.PublicURL)
	adder := pieceadder.New(service, cfg.Server.PublicURL)
	return &Shit{
		aggregator:  agg,
		pieceFinder: finder,
		pieceAdder:  adder,
	}, nil
}

type Params struct {
	fx.In

	DB             *gorm.DB `name:"engine_db"`
	Config         app.AppConfig
	Store          blobstore.PDPStore
	Stash          store.Stash
	Sender         ethereum.Sender
	Engine         *scheduler.TaskEngine
	ChainScheduler *chainsched.Scheduler
}

func ProvidePDPService(params Params) (*service.PDPService, error) {
	return service.NewPDPService(
		params.DB,
		params.Config.Blockchain.OwnerAddr,
		params.Store,
		params.Stash,
		params.Sender,
		params.Engine,
		params.ChainScheduler,
	)
}

type ProofSetProvider struct {
	svc          *service.PDPService
	recordKeeper common.Address
}

func (p *ProofSetProvider) GetOrCreateProofSet(ctx context.Context) (uint64, error) {
	proofSets, err := p.svc.ListProofSets(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing proof sets: %w", err)
	}
	// we need to create one
	if len(proofSets) == 0 {
		createProofSetTxHash, err := p.svc.CreateProofSet(ctx, p.recordKeeper)
		if err != nil {
			return 0, fmt.Errorf("creating proof set: %w", err)
		}
		proofSetID, err := backoff.Retry(ctx, func() (uint64, error) {
			status, err := p.svc.GetProofSetStatus(ctx, createProofSetTxHash)
			if err != nil {
				return 0, backoff.Permanent(fmt.Errorf("getting proof set: %w", err))
			}
			if status.ID != 0 {
				return status.ID, nil
			}
			return 0, fmt.Errorf("proof set not found")
		}, backoff.WithMaxTries(100), backoff.WithBackOff(backoff.NewConstantBackOff(10*time.Second)))
		if err != nil {
			return 0, fmt.Errorf("waiting for proof set to be created: %w", err)
		}
		return proofSetID, nil
	}
	if len(proofSets) == 1 {
		return proofSets[0].ID, nil
	}
	return 0, fmt.Errorf("multiple proof sets found")

}

func ProvideProofSetProvider(svc *service.PDPService, cfg app.AppConfig) *ProofSetProvider {
	return &ProofSetProvider{
		svc:          svc,
		recordKeeper: cfg.Blockchain.RecordKeeperAddr,
	}
}
