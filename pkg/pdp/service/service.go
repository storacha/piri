package service

import (
	"context"
	"net/url"
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

	"github.com/storacha/go-ucanto/ucan"
	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/ethereum"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/tasks"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store/acceptancestore"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/store/receiptstore"
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
	id              ucan.Signer
	endpoint        url.URL
	address         common.Address
	blobstore       blobstore.Blobstore
	acceptanceStore acceptancestore.AcceptanceStore
	receiptStore    receiptstore.ReceiptStore
	sender          ethereum.Sender
	chainClient     ChainClient

	db   *gorm.DB
	name string

	pieceResolver types.PieceResolverAPI
	pieceReader   types.PieceReaderAPI

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
	id ucan.Signer,
	endpoint url.URL,
	db *gorm.DB,
	address common.Address,
	bs blobstore.PDPStore,
	acceptanceStore acceptancestore.AcceptanceStore,
	receiptStore receiptstore.ReceiptStore,
	resolver types.PieceResolverAPI,
	reader types.PieceReaderAPI,
	sender ethereum.Sender,
	engine *scheduler.TaskEngine,
	chainScheduler *chainsched.Scheduler,
	chainClient ChainClient,
	signingService signer.SigningService,
	edc *eip712.ExtraDataEncoder,
	verifier smartcontracts.Verifier,
	serviceContract smartcontracts.Service,
	registryContract smartcontracts.Registry,
) (*PDPService, error) {
	return &PDPService{
		id:               id,
		endpoint:         endpoint,
		address:          address,
		db:               db,
		name:             "storacha",
		pieceResolver:    resolver,
		pieceReader:      reader,
		blobstore:        bs,
		acceptanceStore:  acceptanceStore,
		receiptStore:     receiptStore,
		sender:           sender,
		engine:           engine,
		chainScheduler:   chainScheduler,
		chainClient:      chainClient,
		signingService:   signingService,
		edc:              edc,
		verifierContract: verifier,
		serviceContract:  serviceContract,
		registryContract: registryContract,
	}, nil
}
