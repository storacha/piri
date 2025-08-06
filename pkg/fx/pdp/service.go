package pdp

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v5"
	"go.uber.org/fx"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service"
	"github.com/storacha/piri/pkg/pdp/service/contract"
	"github.com/storacha/piri/pkg/pdp/store"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/wallet"
)

var Module = fx.Module("pdp-service",
	fx.Provide(
		ProvidePDPService,
		ProvideProofSetID,
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

func ProvideProofSetID(svc *service.PDPService, cfg app.AppConfig) (uint64, error) {
	ctx := context.TODO()
	proofSets, err := svc.ListProofSets(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing proof sets: %w", err)
	}
	// we need to create one
	if len(proofSets) == 0 {
		createProofSetTxHash, err := svc.CreateProofSet(ctx, cfg.Blockchain.RecordKeeperAddr)
		if err != nil {
			return 0, fmt.Errorf("creating proof set: %w", err)
		}
		proofSetID, err := backoff.Retry(ctx, func() (uint64, error) {
			status, err := svc.GetProofSetStatus(ctx, createProofSetTxHash)
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
