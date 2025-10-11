package service

import (
	"context"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/api"
	filtypes "github.com/filecoin-project/lotus/chain/types"
	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
	sstypes "github.com/storacha/piri/tools/signing-service/types"
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
	address         common.Address
	blobstore       blobstore.Blobstore
	storage         stashstore.Stash
	sender          ethereum.Sender
	chainClient     ChainClient
	contractClient  smartcontracts.PDP
	contractBackend bind.ContractBackend

	db   *gorm.DB
	name string

	chainScheduler *chainsched.Scheduler
	engine         *scheduler.TaskEngine

	signingService  sstypes.SigningService
	viewContract    *smartcontracts.ViewContractHelper
	extraDataHelper *ExtraDataEncoder
	payerAddress    common.Address // The address that pays for storage
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
	chainClient ChainClient,
	contractClient smartcontracts.PDP,
	contractBackend EthClient,
	signingService sstypes.SigningService,
	payerAddress common.Address,
	serviceContractAddress common.Address,
) (*PDPService, error) {
	// Initialize view contract helper if service contract address is provided
	var viewContract *smartcontracts.ViewContractHelper
	if serviceContractAddress != (common.Address{}) {
		// Assuming contractBackend implements the needed interface for ethclient
		if ethClient, ok := contractBackend.(*ethclient.Client); ok {
			vc, err := smartcontracts.NewViewContractHelper(ethClient, serviceContractAddress)
			if err != nil {
				log.Warnf("Failed to initialize view contract helper: %v", err)
				// Not fatal - continue without view contract
			} else {
				viewContract = vc
			}
		}
	}

	return &PDPService{
		address:         address,
		db:              db,
		name:            "storacha",
		blobstore:       bs,
		storage:         stash,
		sender:          sender,
		engine:          engine,
		chainScheduler:  chainScheduler,
		chainClient:     chainClient,
		contractClient:  contractClient,
		contractBackend: contractBackend,
		signingService:  signingService,
		viewContract:    viewContract,
		extraDataHelper: NewExtraDataEncoder(),
		payerAddress:    payerAddress,
	}, nil
}
