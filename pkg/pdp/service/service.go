package service

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/api"
	filtypes "github.com/filecoin-project/lotus/chain/types"
	"github.com/hashicorp/go-multierror"

	logging "github.com/ipfs/go-log/v2"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/ethereum"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service/contract"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/store"
	"github.com/storacha/piri/pkg/pdp/tasks"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/wallet"
)

var log = logging.Logger("pdp/service")

var _ types.API = (*PDPService)(nil)

type PDPService struct {
	address   common.Address
	blobstore blobstore.Blobstore
	storage   store.Stash
	sender    ethereum.Sender

	db   *gorm.DB
	name string

	chainScheduler *chainsched.Scheduler
	engine         *scheduler.TaskEngine

	stopFns  []func(ctx context.Context) error
	startFns []func(ctx context.Context) error
}

func (p *PDPService) Storage() blobstore.Blobstore {
	return p.blobstore
}

func (p *PDPService) Start(ctx context.Context) error {
	for _, startFn := range p.startFns {
		if err := startFn(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (p *PDPService) Stop(ctx context.Context) error {
	var errs error
	for _, stopFn := range p.stopFns {
		if err := stopFn(ctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

type ChainClient interface {
	ChainHead(ctx context.Context) (*filtypes.TipSet, error)
	ChainNotify(ctx context.Context) (<-chan []*api.HeadChange, error)
	StateGetRandomnessDigestFromBeacon(ctx context.Context, randEpoch abi.ChainEpoch, tsk filtypes.TipSetKey) (abi.Randomness, error)
}

type EthClient interface {
	tasks.SenderETHClient
	tasks.MessageWatcherEthClient
	bind.ContractBackend
}

func NewPDPService(
	db *gorm.DB,
	address common.Address,
	wallet wallet.Wallet,
	bs blobstore.Blobstore,
	ss store.Stash,
	chainClient ChainClient,
	ethClient EthClient,
	contractClient contract.PDP,
) (*PDPService, error) {
	var (
		startFns []func(context.Context) error
		stopFns  []func(context.Context) error
	)
	// apply the PDP service database models to the database.
	if err := models.AutoMigrateDB(db); err != nil {
		return nil, err
	}
	chainScheduler := chainsched.New(chainClient)

	var t []scheduler.TaskInterface
	sender, senderTask := tasks.NewSenderETH(ethClient, wallet, db)
	t = append(t, senderTask)

	pdpInitTask, err := tasks.NewInitProvingPeriodTask(db, ethClient, contractClient, chainClient, chainScheduler, sender)
	if err != nil {
		return nil, fmt.Errorf("creating init proving period task: %w", err)
	}
	t = append(t, pdpInitTask)

	pdpNextTask, err := tasks.NewNextProvingPeriodTask(db, ethClient, contractClient, chainClient, chainScheduler, sender)
	if err != nil {
		return nil, fmt.Errorf("creating next proving period task: %w", err)
	}
	t = append(t, pdpNextTask)

	pdpNotifyTask := tasks.NewPDPNotifyTask(db)
	t = append(t, pdpNotifyTask)

	pdpProveTask, err := tasks.NewProveTask(chainScheduler, db, ethClient, contractClient, chainClient, sender, bs)
	if err != nil {
		return nil, fmt.Errorf("creating prove period task: %w", err)
	}
	t = append(t, pdpProveTask)

	if err := tasks.NewWatcherCreate(db, ethClient, contractClient, chainScheduler); err != nil {
		return nil, fmt.Errorf("creating watcher root create: %w", err)
	}

	if err := tasks.NewWatcherRootAdd(db, chainScheduler, contractClient); err != nil {
		return nil, fmt.Errorf("creating watcher root add: %w", err)
	}

	// TODO this needs configuration and or tuning
	maxMoveStoreTasks := 8
	pdpStorePieceTask, err := tasks.NewStorePieceTask(db, bs, maxMoveStoreTasks)
	if err != nil {
		return nil, fmt.Errorf("creating pdp store piece task: %w", err)
	}
	t = append(t, pdpStorePieceTask)

	engine, err := scheduler.NewEngine(db, t)
	if err != nil {
		return nil, fmt.Errorf("creating engine: %w", err)
	}
	stopFns = append(stopFns, func(ctx context.Context) error {
		engine.GracefullyTerminate()
		return nil
	})

	// TODO this needs to be manually stopped
	ethWatcher, err := tasks.NewMessageWatcherEth(db, chainScheduler, ethClient)
	if err != nil {
		return nil, fmt.Errorf("creating message watcher: %w", err)
	}

	startFns = append(startFns, func(ctx context.Context) error {
		go chainScheduler.Run(ctx)
		return nil
	})
	stopFns = append(stopFns, ethWatcher.Stop)

	return &PDPService{
		address:        address,
		db:             db,
		name:           "storacha",
		blobstore:      bs,
		storage:        ss,
		sender:         sender,
		startFns:       startFns,
		stopFns:        stopFns,
		engine:         engine,
		chainScheduler: chainScheduler,
	}, nil
}
