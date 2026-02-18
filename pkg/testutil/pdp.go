// Package testutil provides testing utilities for PDP integration tests.
package testutil

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/client"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/storacha/filecoin-services/go/bindings"
	"github.com/storacha/filecoin-services/go/eip712"
	libstorachatestutil "github.com/storacha/go-libstoracha/testutil"
	signerimpl "github.com/storacha/piri-signing-service/pkg/inprocess"
	signingservice "github.com/storacha/piri-signing-service/pkg/signer"
	signertypes "github.com/storacha/piri-signing-service/pkg/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	appconfig "github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/database"
	"github.com/storacha/piri/pkg/database/gormdb"
	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/piece"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
	"github.com/storacha/piri/pkg/pdp/tasks"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store/acceptancestore"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/store/keystore"
	"github.com/storacha/piri/pkg/store/receiptstore"
	"github.com/storacha/piri/pkg/testutil/localdev"
	"github.com/storacha/piri/pkg/wallet"
)

// PDPTestHarness encapsulates all test dependencies for PDP integration testing.
type PDPTestHarness struct {
	T      *testing.T
	Ctx    context.Context
	Cancel context.CancelFunc

	// Container is the localdev container running Anvil with pre-deployed contracts
	Container *localdev.Container

	// Clients (both connect to same container endpoint)
	EthClient  *ethclient.Client
	LotusAPI   api.FullNode
	lotusClose func()
	AnvilCtl   *AnvilClient

	// Database
	DB     *gorm.DB
	dbPath string

	// Core services
	PDPService     *service.PDPService
	Engine         *scheduler.TaskEngine
	ChainScheduler *chainsched.Scheduler
	Wallet         wallet.Wallet
	Sender         *tasks.SenderETH

	// Smart contracts
	RegistryContract smartcontracts.Registry
	VerifierContract smartcontracts.Verifier
	ServiceContract  smartcontracts.Service

	// Signing service (uses Payer key)
	SigningService signertypes.SigningService

	// Stores
	BlobStore       blobstore.PDPStore
	AcceptanceStore acceptancestore.AcceptanceStore
	ReceiptStore    receiptstore.ReceiptStore
	PieceResolver   types.PieceResolverAPI
	PieceReader     types.PieceReaderAPI

	// Keys
	ServiceProviderKey *ecdsa.PrivateKey
	PayerKey           *ecdsa.PrivateKey
	DeployerKey        *ecdsa.PrivateKey

	// Test state
	ProviderID uint64
	ProofSetID uint64
}

// SkipIfNotIntegration skips the test if conditions for integration testing are not met.
func SkipIfNotIntegration(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if runtime.GOOS == "darwin" {
		t.Skip("skipping on darwin - testcontainers may not work in CI")
	}
}

// NewPDPTestHarness creates a new test harness with the localdev container.
// It initializes all necessary clients and services for PDP testing.
func NewPDPTestHarness(t *testing.T) *PDPTestHarness {
	t.Helper()
	SkipIfNotIntegration(t)

	ctx, cancel := context.WithCancel(context.Background())

	// Start localdev container
	container, err := localdev.Run(ctx,
		localdev.WithImage("filecoin-localdev:local"),
		localdev.WithBlockTime(3),                  // Minimum block time is 3 seconds
		localdev.WithStartupTimeout(3*time.Minute), // Allow ample time for contract deployment
		localdev.WithStateFile("/home/frrist/workspace/src/github.com/storacha/piri/pkg/testutil/anvil-state.json"),
		localdev.WithDeployedAddressesFile("/home/frrist/workspace/src/github.com/storacha/piri/pkg/testutil/deployed-addresses.json"),
	)
	require.NoError(t, err, "failed to start localdev container")

	t.Logf("Localdev container started at %s", container.RPCEndpoint)

	// Parse private keys
	serviceProviderKey, err := crypto.HexToECDSA(strings.TrimPrefix(localdev.Accounts.ServiceProvider.PrivateKey, "0x"))
	require.NoError(t, err)

	payerKey, err := crypto.HexToECDSA(strings.TrimPrefix(localdev.Accounts.Payer.PrivateKey, "0x"))
	require.NoError(t, err)

	deployerKey, err := crypto.HexToECDSA(strings.TrimPrefix(localdev.Accounts.Deployer.PrivateKey, "0x"))
	require.NoError(t, err)

	h := &PDPTestHarness{
		T:                  t,
		Ctx:                ctx,
		Cancel:             cancel,
		Container:          container,
		ServiceProviderKey: serviceProviderKey,
		PayerKey:           payerKey,
		DeployerKey:        deployerKey,
	}

	t.Cleanup(func() {
		h.Stop()
	})

	return h
}

// Start initializes all services and starts the scheduler.
func (h *PDPTestHarness) Start() error {
	var err error

	// Connect Ethereum client
	h.EthClient, err = ethclient.Dial(h.Container.RPCEndpoint)
	if err != nil {
		return fmt.Errorf("connecting eth client: %w", err)
	}

	// Connect Lotus client using WebSocket (required for ChainNotify subscriptions)
	// Container handles both HTTP (eth_*) and WebSocket (Filecoin.ChainNotify) on same port
	wsEndpoint := strings.Replace(h.Container.RPCEndpoint, "http://", "ws://", 1)
	h.LotusAPI, h.lotusClose, err = client.NewFullNodeRPCV1(h.Ctx, wsEndpoint, nil)
	if err != nil {
		return fmt.Errorf("connecting lotus client: %w", err)
	}

	// Create Anvil control client
	h.AnvilCtl, err = NewAnvilClient(h.Container.RPCEndpoint)
	if err != nil {
		return fmt.Errorf("creating anvil client: %w", err)
	}

	// Create temporary database
	tempDir, err := os.MkdirTemp("", "pdp-test-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	h.dbPath = filepath.Join(tempDir, "pdp-test.db")

	h.DB, err = gormdb.New(h.dbPath,
		database.WithForeignKeyConstraintsEnable(true),
		database.WithTimeout(5*time.Second),
	)
	if err != nil {
		return fmt.Errorf("creating database: %w", err)
	}

	// Run migrations using the full AutoMigrateDB which also installs triggers
	if err := models.AutoMigrateDB(h.Ctx, h.DB); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	// Create smart contract instances using dynamically loaded addresses from container
	h.RegistryContract, err = smartcontracts.NewRegistry(
		common.HexToAddress(h.Container.Addresses.ServiceProviderRegistry),
		h.EthClient,
	)
	if err != nil {
		return fmt.Errorf("creating registry contract: %w", err)
	}

	h.VerifierContract, err = smartcontracts.NewVerifierContract(
		common.HexToAddress(h.Container.Addresses.PDPVerifier),
		h.EthClient,
	)
	if err != nil {
		return fmt.Errorf("creating verifier contract: %w", err)
	}

	h.ServiceContract, err = smartcontracts.NewServiceView(
		common.HexToAddress(h.Container.Addresses.ServiceStateView),
		h.EthClient,
	)
	if err != nil {
		return fmt.Errorf("creating service contract: %w", err)
	}

	// Create wallet with ServiceProvider key
	ks := keystore.NewMemKeyStore()
	h.Wallet, err = wallet.NewWallet(ks)
	if err != nil {
		return fmt.Errorf("creating wallet: %w", err)
	}

	// Import ServiceProvider key into wallet
	ki := &keystore.KeyInfo{PrivateKey: crypto.FromECDSA(h.ServiceProviderKey)}
	if _, err := h.Wallet.Import(h.Ctx, ki); err != nil {
		return fmt.Errorf("importing service provider key: %w", err)
	}

	// Create chain scheduler (monitors Filecoin chain)
	h.ChainScheduler, err = chainsched.New(h.LotusAPI)
	if err != nil {
		return fmt.Errorf("creating chain scheduler: %w", err)
	}

	// Create sender and send task (send task must be added to scheduler)
	var sendTask scheduler.TaskInterface
	h.Sender, sendTask, err = tasks.NewSenderETH(h.EthClient, h.Wallet, h.DB)
	if err != nil {
		return fmt.Errorf("creating sender: %w", err)
	}

	// Create signing service with Payer key
	signer := signingservice.NewSigner(
		h.PayerKey,
		big.NewInt(localdev.ChainID),
		common.HexToAddress(h.Container.Addresses.FilecoinWarmStorageService),
	)
	h.SigningService = signerimpl.New(signer)

	// Create in-memory stores using datastore-backed implementations
	ds := dssync.MutexWrap(datastore.NewMapDatastore())
	h.BlobStore = blobstore.NewTODO_DsBlobstore(ds)
	h.AcceptanceStore, err = acceptancestore.NewDsAcceptanceStore(ds)
	if err != nil {
		return fmt.Errorf("creating acceptance store: %w", err)
	}
	h.ReceiptStore, err = receiptstore.NewDsReceiptStore(ds)
	if err != nil {
		return fmt.Errorf("creating receipt store: %w", err)
	}

	// Create piece resolver and reader
	h.PieceResolver, err = piece.NewStoreResolver(piece.StoreResolverParams{DB: h.DB})
	if err != nil {
		return fmt.Errorf("creating piece resolver: %w", err)
	}
	h.PieceReader, err = piece.NewStoreReader(h.BlobStore)
	if err != nil {
		return fmt.Errorf("creating piece reader: %w", err)
	}

	// Create EIP-712 encoder
	edc := eip712.NewExtraDataEncoder()

	// Create task engine (but don't start yet - need to add tasks first)
	schedulerTasks := []scheduler.TaskInterface{sendTask} // Send task is essential for processing transactions

	// Create proving tasks
	initTask, err := tasks.NewInitProvingPeriodTask(
		h.DB, h.EthClient, h.LotusAPI, h.ChainScheduler, h.Sender, h.ServiceContract, h.VerifierContract,
	)
	if err != nil {
		return fmt.Errorf("creating init proving period task: %w", err)
	}
	schedulerTasks = append(schedulerTasks, initTask)

	nextTask, err := tasks.NewNextProvingPeriodTask(
		h.DB, h.EthClient, h.LotusAPI, h.ChainScheduler, h.Sender, h.VerifierContract, h.ServiceContract,
	)
	if err != nil {
		return fmt.Errorf("creating next proving period task: %w", err)
	}
	schedulerTasks = append(schedulerTasks, nextTask)

	proveTask, err := tasks.NewProveTask(
		h.ChainScheduler, h.DB, h.EthClient, h.VerifierContract, h.LotusAPI, h.Sender, h.BlobStore, h.PieceReader, h.PieceResolver,
	)
	if err != nil {
		return fmt.Errorf("creating prove task: %w", err)
	}
	schedulerTasks = append(schedulerTasks, proveTask)

	h.Engine, err = scheduler.NewEngine(h.DB, schedulerTasks)
	if err != nil {
		return fmt.Errorf("creating task engine: %w", err)
	}

	// Configure PDP service using container addresses
	serviceProviderAddr := common.HexToAddress(localdev.Accounts.ServiceProvider.Address)
	pdpCfg := appconfig.PDPServiceConfig{
		OwnerAddress: serviceProviderAddr,
		PayerAddress: common.HexToAddress(localdev.Accounts.Payer.Address),
		ChainID:      big.NewInt(localdev.ChainID),
		Contracts: appconfig.ContractAddresses{
			Verifier:         common.HexToAddress(h.Container.Addresses.PDPVerifier),
			ProviderRegistry: common.HexToAddress(h.Container.Addresses.ServiceProviderRegistry),
			Service:          common.HexToAddress(h.Container.Addresses.FilecoinWarmStorageService),
			ServiceView:      common.HexToAddress(h.Container.Addresses.ServiceStateView),
		},
	}

	publicURL, _ := url.Parse("http://localhost:8080")

	// Create PDP service
	h.PDPService, err = service.New(
		pdpCfg,
		libstorachatestutil.Alice, // UCAN signer for identity
		*publicURL,
		h.DB,
		h.BlobStore,
		h.AcceptanceStore,
		h.ReceiptStore,
		h.PieceResolver,
		h.PieceReader,
		h.Sender,
		h.Engine,
		h.ChainScheduler,
		h.LotusAPI,
		h.SigningService,
		edc,
		h.VerifierContract,
		h.ServiceContract,
		h.RegistryContract,
	)
	if err != nil {
		return fmt.Errorf("creating PDP service: %w", err)
	}

	// Start watchers
	msgWatcher, err := tasks.NewMessageWatcherEth(h.DB, h.ChainScheduler, h.EthClient)
	if err != nil {
		return fmt.Errorf("creating message watcher: %w", err)
	}
	msgWatcher.Start() // Must call Start() to begin processing transactions
	if err := tasks.NewWatcherCreate(h.DB, h.VerifierContract, h.ChainScheduler, h.ServiceContract); err != nil {
		return fmt.Errorf("creating create watcher: %w", err)
	}
	if err := tasks.NewWatcherRootAdd(h.DB, h.ChainScheduler, h.VerifierContract); err != nil {
		return fmt.Errorf("creating root add watcher: %w", err)
	}
	if err := tasks.NewWatcherProviderRegister(h.DB, h.ChainScheduler, h.RegistryContract.Address()); err != nil {
		return fmt.Errorf("creating provider register watcher: %w", err)
	}

	// Start chain scheduler
	go h.ChainScheduler.Run(h.Ctx)

	// Start task engine
	if err := h.Engine.Start(h.Ctx); err != nil {
		return fmt.Errorf("starting task engine: %w", err)
	}

	return nil
}

// Stop cleans up all resources.
func (h *PDPTestHarness) Stop() {
	if h.Engine != nil {
		_ = h.Engine.Stop(context.Background())
	}
	if h.Cancel != nil {
		h.Cancel()
	}
	if h.AnvilCtl != nil {
		h.AnvilCtl.Close()
	}
	if h.lotusClose != nil {
		h.lotusClose()
	}
	if h.EthClient != nil {
		h.EthClient.Close()
	}
	if h.DB != nil {
		sqlDB, _ := h.DB.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	if h.dbPath != "" {
		_ = os.RemoveAll(filepath.Dir(h.dbPath))
	}
	if h.Container != nil {
		_ = h.Container.Terminate(context.Background())
	}
}

// MineBlocks advances the chain by mining n blocks.
func (h *PDPTestHarness) MineBlocks(n int) error {
	return h.AnvilCtl.MineBlocks(n)
}

// MineBlock mines a single block.
func (h *PDPTestHarness) MineBlock() error {
	return h.AnvilCtl.MineBlock()
}

// WaitForTxConfirmation waits for a transaction to be mined and confirmed.
// It actively mines blocks to help the transaction get included.
func (h *PDPTestHarness) WaitForTxConfirmation(txHash common.Hash, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(h.Ctx, timeout)
	defer cancel()

	for {
		receipt, err := h.EthClient.TransactionReceipt(ctx, txHash)
		if err == nil && receipt != nil {
			if receipt.Status == ethtypes.ReceiptStatusFailed {
				return fmt.Errorf("transaction %s failed", txHash.Hex())
			}
			return nil
		}

		// Mine a block to help confirmation
		_ = h.AnvilCtl.MineBlock()

		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for tx %s: %w", txHash.Hex(), ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// ApproveProvider calls AddApprovedProvider on the ServiceContract.
// Must be called by the contract owner (Deployer #0) after provider registration.
func (h *PDPTestHarness) ApproveProvider(providerID uint64) error {
	// Create transactor with Deployer's private key
	auth, err := bind.NewKeyedTransactorWithChainID(h.DeployerKey, big.NewInt(localdev.ChainID))
	if err != nil {
		return fmt.Errorf("creating transactor: %w", err)
	}

	// Get FilecoinWarmStorageService contract binding using container addresses
	serviceContract, err := bindings.NewFilecoinWarmStorageService(
		common.HexToAddress(h.Container.Addresses.FilecoinWarmStorageService),
		h.EthClient,
	)
	if err != nil {
		return fmt.Errorf("creating service contract binding: %w", err)
	}

	// Call AddApprovedProvider
	tx, err := serviceContract.AddApprovedProvider(auth, big.NewInt(int64(providerID)))
	if err != nil {
		return fmt.Errorf("calling AddApprovedProvider: %w", err)
	}

	return h.WaitForTxConfirmation(tx.Hash(), 30*time.Second)
}

// AdvanceToEpoch advances the chain to the target epoch by mining blocks.
func (h *PDPTestHarness) AdvanceToEpoch(targetEpoch int64) error {
	head, err := h.LotusAPI.ChainHead(h.Ctx)
	if err != nil {
		return fmt.Errorf("getting chain head: %w", err)
	}
	current := int64(head.Height())

	if targetEpoch > current {
		blocksNeeded := targetEpoch - current
		h.T.Logf("Advancing chain from epoch %d to %d (mining %d blocks)", current, targetEpoch, blocksNeeded)
		return h.AnvilCtl.MineBlocks(int(blocksNeeded))
	}
	return nil
}

// CurrentEpoch returns the current chain epoch.
func (h *PDPTestHarness) CurrentEpoch() (int64, error) {
	head, err := h.LotusAPI.ChainHead(h.Ctx)
	if err != nil {
		return 0, err
	}
	return int64(head.Height()), nil
}

// WaitForDBCondition polls the database until the condition function returns true.
func (h *PDPTestHarness) WaitForDBCondition(condition func(*gorm.DB) bool, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(h.Ctx, timeout)
	defer cancel()

	for {
		if condition(h.DB) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// WaitForProofSet waits for at least one proof set to appear in the database.
// This is useful after creating a proof set to wait for the watcher to process the event.
// Note: The MessageWatcherEth requires MinConfidence=6 blocks before marking transactions confirmed.
func (h *PDPTestHarness) WaitForProofSet(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(h.Ctx, timeout)
	defer cancel()

	for {
		proofSets, err := h.PDPService.ListProofSets(h.Ctx)
		if err == nil && len(proofSets) > 0 {
			return nil
		}

		// Mine multiple blocks to clear MinConfidence threshold (6 blocks)
		// and trigger watcher processing via chain scheduler
		_ = h.AnvilCtl.MineBlocks(10)

		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for proof set to appear: %w", ctx.Err())
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// WaitForProofSetCount waits for at least count proof sets to appear in the database.
// Note: The MessageWatcherEth requires MinConfidence=6 blocks before marking transactions confirmed.
func (h *PDPTestHarness) WaitForProofSetCount(count int, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(h.Ctx, timeout)
	defer cancel()

	var lastCount int
	for {
		proofSets, err := h.PDPService.ListProofSets(h.Ctx)
		if err == nil {
			lastCount = len(proofSets)
			if lastCount >= count {
				return nil
			}
		}

		// Mine multiple blocks to clear MinConfidence threshold (6 blocks)
		// and trigger watcher processing via chain scheduler
		_ = h.AnvilCtl.MineBlocks(10)

		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for %d proof sets (have %d): %w", count, lastCount, ctx.Err())
		case <-time.After(500 * time.Millisecond):
		}
	}
}
