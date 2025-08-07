package service

import (
	"context"

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
	"github.com/storacha/piri/pkg/pdp/store"
	"github.com/storacha/piri/pkg/pdp/tasks"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store/blobstore"
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
	bs blobstore.PDPStore,
	ss store.Stash,
	sender ethereum.Sender,
	engine *scheduler.TaskEngine,
	chainScheduler *chainsched.Scheduler,
) (*PDPService, error) {
	return &PDPService{
		address:        address,
		db:             db,
		name:           "storacha",
		blobstore:      bs,
		storage:        ss,
		sender:         sender,
		engine:         engine,
		chainScheduler: chainScheduler,
	}, nil
}
