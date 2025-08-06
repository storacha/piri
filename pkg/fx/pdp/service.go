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
	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service"
	"github.com/storacha/piri/pkg/pdp/service/contract"
	"github.com/storacha/piri/pkg/pdp/store"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/wallet"
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
	),
)

type Params struct {
	fx.In

	Config         app.AppConfig
	Wallet         wallet.Wallet
	Store          blobstore.PDPStore
	Stash          store.Stash
	ChainClient    service.ChainClient
	EthClient      service.EthClient
	ContractClient contract.PDP

	DB             *gorm.DB `name:"engine_db"`
	ChainScheduler *chainsched.Scheduler
	TaskEngine     *scheduler.TaskEngine
}

func ProvidePDPService(params Params) (*service.PDPService, error) {
	return service.NewPDPService(
		params.DB,
		params.Config.Blockchain.OwnerAddr,
		params.Wallet,
		params.Store,
		params.Stash,
		params.ChainClient,
		params.EthClient,
		params.ContractClient,
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
		recordKeeper: cfg.Blockchain.OwnerAddr,
	}
}
