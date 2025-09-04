package service

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/api"
	filtypes "github.com/filecoin-project/lotus/chain/types"
	logging "github.com/ipfs/go-log/v2"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/ethereum"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/tasks"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/store/stashstore"
)

var log = logging.Logger("pdp/service")

var _ types.API = (*PDPService)(nil)

type PDPService struct {
	address   common.Address
	blobstore blobstore.Blobstore
	storage   stashstore.Stash
	sender    ethereum.Sender

	db   *gorm.DB
	name string

	chainScheduler  *chainsched.Scheduler
	engine          *scheduler.TaskEngine
	activeUploads   *atomic.Int32
	activeDownloads *atomic.Int32

	stopFns  []func(ctx context.Context) error
	startFns []func(ctx context.Context) error
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

func New(
	db *gorm.DB,
	address common.Address,
	bs blobstore.PDPStore,
	stash stashstore.Stash,
	sender ethereum.Sender,
	engine *scheduler.TaskEngine,
	chainScheduler *chainsched.Scheduler,
) (*PDPService, error) {
	return &PDPService{
		address:         address,
		db:              db,
		name:            "storacha",
		blobstore:       bs,
		storage:         stash,
		sender:          sender,
		engine:          engine,
		chainScheduler:  chainScheduler,
		activeUploads:   new(atomic.Int32),
		activeDownloads: new(atomic.Int32),
	}, nil
}

// Stop gracefully shuts down the PDPService, waiting for active operations to complete
func (p *PDPService) Stop(ctx context.Context) error {
	log.Infow("PDPService stopping, waiting for active operations",
		"active_uploads", p.activeUploads.Load(),
		"active_downloads", p.activeDownloads.Load())

	// Poll for completion of all uploads and downloads
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Context timeout - log any remaining operations
			uploads := p.activeUploads.Load()
			downloads := p.activeDownloads.Load()
			if uploads > 0 || downloads > 0 {
				log.Errorf("PDPService stop timeout with active operations",
					"active_uploads", uploads,
					"active_downloads", downloads)
			}
			return fmt.Errorf("stop timeout: %w", ctx.Err())
		case <-ticker.C:
			uploads := p.activeUploads.Load()
			downloads := p.activeDownloads.Load()
			
			if uploads == 0 && downloads == 0 {
				log.Infow("PDPService stopped successfully, all operations completed")
				return nil
			}
			
			// Log progress periodically
			log.Debugw("Waiting for operations to complete",
				"active_uploads", uploads,
				"active_downloads", downloads)
		}
	}
}
