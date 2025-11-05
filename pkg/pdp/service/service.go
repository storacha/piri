package service

import (
	"context"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/api"
	filtypes "github.com/filecoin-project/lotus/chain/types"
	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/filecoin-services/go/eip712"
	signer "github.com/storacha/piri-signing-service/pkg/types"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/ethereum"
	"github.com/storacha/piri/pkg/pdp/pieces"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/tasks"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/store/stashstore"
)

var log = logging.Logger("pdp/service")

var _ types.API = (*PDPService)(nil)

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

type PDPService struct {
	address         common.Address
	blobstore       blobstore.Blobstore
	storage         stashstore.Stash
	sender          ethereum.Sender
	chainClient     ChainClient
	contractBackend bind.ContractBackend

	db   *gorm.DB
	name string

	pieceResolver pieces.Resolver

	chainScheduler *chainsched.Scheduler
	engine         *scheduler.TaskEngine
	signingService signer.SigningService

	edc              *eip712.ExtraDataEncoder
	verifierContract smartcontracts.Verifier
	serviceContract  smartcontracts.Service
	registryContract smartcontracts.Registry

	addRootMu sync.Mutex
}

func New(
	db *gorm.DB,
	address common.Address,
	bs blobstore.PDPStore,
	resolver pieces.Resolver,
	stash stashstore.Stash,
	sender ethereum.Sender,
	engine *scheduler.TaskEngine,
	chainScheduler *chainsched.Scheduler,
	chainClient ChainClient,
	contractBackend EthClient,
	signingService signer.SigningService,
	edc *eip712.ExtraDataEncoder,
	verifier smartcontracts.Verifier,
	serviceContract smartcontracts.Service,
	registryContract smartcontracts.Registry,
) (*PDPService, error) {
	return &PDPService{
		address:          address,
		db:               db,
		name:             "storacha",
		blobstore:        bs,
		pieceResolver:    resolver,
		storage:          stash,
		sender:           sender,
		engine:           engine,
		chainScheduler:   chainScheduler,
		chainClient:      chainClient,
		contractBackend:  contractBackend,
		signingService:   signingService,
		edc:              edc,
		verifierContract: verifier,
		serviceContract:  serviceContract,
		registryContract: registryContract,
	}, nil
}
