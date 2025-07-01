package tasks

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/database/gormdb"
	"github.com/storacha/piri/pkg/pdp/service/models"
)

// fakeEthClient implements MessageWatcherEthClient for testing
type fakeEthClient struct {
	mu               sync.RWMutex
	receiptResponses map[common.Hash]*receiptResponse
	txResponses      map[common.Hash]*txResponse
	callCount        map[string]*atomic.Int32
}

type receiptResponse struct {
	receipt      *types.Receipt
	err          error
	failureCount int // Number of times to fail before succeeding
}

type txResponse struct {
	tx           *types.Transaction
	isPending    bool
	err          error
	failureCount int
}

func newFakeEthClient() *fakeEthClient {
	return &fakeEthClient{
		receiptResponses: make(map[common.Hash]*receiptResponse),
		txResponses:      make(map[common.Hash]*txResponse),
		callCount: map[string]*atomic.Int32{
			"TransactionReceipt": {},
			"TransactionByHash":  {},
		},
	}
}

func (m *fakeEthClient) TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	m.callCount["TransactionReceipt"].Add(1)

	m.mu.Lock()
	resp, exists := m.receiptResponses[txHash]
	if !exists {
		m.mu.Unlock()
		return nil, ethereum.NotFound
	}

	// Simulate failures before success
	if resp.failureCount > 0 {
		resp.failureCount--
		m.mu.Unlock()
		return nil, errors.New("timeout: i/o timeout")
	}
	m.mu.Unlock()

	return resp.receipt, resp.err
}

func (m *fakeEthClient) TransactionByHash(ctx context.Context, hash common.Hash) (*types.Transaction, bool, error) {
	m.callCount["TransactionByHash"].Add(1)

	m.mu.Lock()
	defer m.mu.Unlock()

	resp, exists := m.txResponses[hash]
	if !exists {
		return nil, false, ethereum.NotFound
	}

	// Simulate failures before success
	if resp.failureCount > 0 {
		resp.failureCount--
		return nil, false, errors.New("timeout: i/o timeout")
	}

	return resp.tx, resp.isPending, resp.err
}

func (m *fakeEthClient) addReceipt(txHash common.Hash, receipt *types.Receipt, failureCount int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.receiptResponses[txHash] = &receiptResponse{
		receipt:      receipt,
		failureCount: failureCount,
	}
}

func (m *fakeEthClient) addTransaction(txHash common.Hash, tx *types.Transaction, failureCount int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.txResponses[txHash] = &txResponse{
		tx:           tx,
		failureCount: failureCount,
	}
}

func (m *fakeEthClient) getCallCount(method string) int32 {
	return m.callCount[method].Load()
}

// Test helper functions
func setupTestDB(t *testing.T) *gorm.DB {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	db, err := gormdb.New(dbPath)
	require.NoError(t, err)

	err = models.AutoMigrateDB(db)
	require.NoError(t, err)

	return db
}

func createTestReceipt(blockNumber int64, status uint64) *types.Receipt {
	return &types.Receipt{
		BlockNumber: big.NewInt(blockNumber),
		Status:      status,
		TxHash:      common.HexToHash(fmt.Sprintf("0x%064d", blockNumber)),
	}
}

func createTestTransaction(nonce uint64) *types.Transaction {
	return types.NewTransaction(
		nonce,
		common.HexToAddress("0x1234567890123456789012345678901234567890"),
		big.NewInt(1000),
		21000,
		big.NewInt(1),
		[]byte("test"),
	)
}

// Unit Tests

func TestGetReceiptWithRetry_Success(t *testing.T) {
	client := newFakeEthClient()
	mw := &MessageWatcherEth{
		api:              client,
		maxEthAPIRetries: 3,
	}

	txHash := common.HexToHash("0x123")
	receipt := createTestReceipt(100, 1)
	client.addReceipt(txHash, receipt, 0) // No failures

	ctx := context.Background()
	result, err := mw.getReceiptWithRetry(ctx, txHash)

	require.NoError(t, err)
	assert.Equal(t, receipt, result)
	assert.Equal(t, int32(1), client.getCallCount("TransactionReceipt"))
}

func TestGetReceiptWithRetry_SuccessAfterRetries(t *testing.T) {
	client := newFakeEthClient()
	mw := &MessageWatcherEth{
		api:              client,
		maxEthAPIRetries: 3,
	}

	txHash := common.HexToHash("0x123")
	receipt := createTestReceipt(100, 1)
	client.addReceipt(txHash, receipt, 2) // Fail twice, then succeed

	ctx := context.Background()
	result, err := mw.getReceiptWithRetry(ctx, txHash)

	require.NoError(t, err)
	assert.Equal(t, receipt, result)
	assert.Equal(t, int32(3), client.getCallCount("TransactionReceipt"))
}

func TestGetReceiptWithRetry_MaxRetriesExceeded(t *testing.T) {
	client := newFakeEthClient()
	mw := &MessageWatcherEth{
		api:              client,
		maxEthAPIRetries: 3,
	}

	txHash := common.HexToHash("0x123")
	receipt := createTestReceipt(100, 1)
	client.addReceipt(txHash, receipt, 5) // More failures than max retries

	ctx := context.Background()
	result, err := mw.getReceiptWithRetry(ctx, txHash)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, int32(mw.maxEthAPIRetries), client.getCallCount("TransactionReceipt"))
}

func TestGetReceiptWithRetry_NotFoundNoRetry(t *testing.T) {
	client := newFakeEthClient()
	mw := &MessageWatcherEth{
		api:              client,
		maxEthAPIRetries: 3,
	}

	txHash := common.HexToHash("0x123")
	// Don't add receipt - will return NotFound

	ctx := context.Background()
	result, err := mw.getReceiptWithRetry(ctx, txHash)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ethereum.NotFound))
	assert.Nil(t, result)
	assert.Equal(t, int32(1), client.getCallCount("TransactionReceipt"))
}

func TestCheckTransaction_Success(t *testing.T) {
	client := newFakeEthClient()
	mw := &MessageWatcherEth{
		api:              client,
		maxEthAPIRetries: 3,
	}

	txHash := common.HexToHash("0x123")
	receipt := createTestReceipt(100, 1)
	tx := createTestTransaction(1)

	client.addReceipt(txHash, receipt, 0)
	client.addTransaction(txHash, tx, 0)

	ctx := context.Background()
	bestBlockNumber := big.NewInt(105) // More than MinConfidence ahead

	result, err := mw.checkTransaction(ctx, txHash, bestBlockNumber)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, txHash.Hex(), result.TxHash)
	assert.Equal(t, receipt, result.Receipt)
	assert.Equal(t, tx, result.Transaction)
	assert.Equal(t, int64(100), result.ConfirmedBlockNumber)
	assert.True(t, result.TxSuccess)
	assert.NotEmpty(t, result.TxDataJSON)
	assert.NotEmpty(t, result.ReceiptJSON)
}

func TestCheckTransaction_InsufficientConfirmations(t *testing.T) {
	client := newFakeEthClient()
	mw := &MessageWatcherEth{
		api:              client,
		maxEthAPIRetries: 3,
	}

	txHash := common.HexToHash("0x123")
	receipt := createTestReceipt(100, 1)

	client.addReceipt(txHash, receipt, 0)

	ctx := context.Background()
	bestBlockNumber := big.NewInt(101) // Only 1 confirmation

	result, err := mw.checkTransaction(ctx, txHash, bestBlockNumber)

	require.NoError(t, err)
	assert.Nil(t, result)
}

// Integration Tests

func TestUpdate_ConcurrentProcessing(t *testing.T) {
	db := setupTestDB(t)
	client := newFakeEthClient()

	mw := &MessageWatcherEth{
		db:               db,
		api:              client,
		maxEthAPIRetries: 3,
	}

	// Set best block number
	mw.bestBlockNumber.Store(big.NewInt(1000))

	// Create pending transactions in database
	numTxs := 20
	for i := 0; i < numTxs; i++ {
		txHash := common.HexToHash(fmt.Sprintf("0x%064d", i))
		err := db.Create(&models.MessageWaitsEth{
			SignedTxHash: txHash.Hex(),
			TxStatus:     "pending",
		}).Error
		require.NoError(t, err)

		// Add mock responses
		receipt := createTestReceipt(int64(900+i), 1)
		tx := createTestTransaction(uint64(i))
		client.addReceipt(txHash, receipt, 0)
		client.addTransaction(txHash, tx, 0)
	}

	// Measure execution time
	start := time.Now()
	mw.update()
	duration := time.Since(start)

	// Verify all transactions were updated
	var confirmedCount int64
	err := db.Model(&models.MessageWaitsEth{}).
		Where("tx_status = ?", "confirmed").
		Count(&confirmedCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(numTxs), confirmedCount)

	// Verify concurrent execution (should be faster than sequential)
	// With maxConcurrentChecks=10 and minimal delays, should complete quickly
	assert.Less(t, duration, 1*time.Second)
}

func TestUpdate_ErrorResilience(t *testing.T) {
	db := setupTestDB(t)
	client := newFakeEthClient()

	mw := &MessageWatcherEth{
		db:               db,
		api:              client,
		maxEthAPIRetries: 3,
	}

	mw.bestBlockNumber.Store(big.NewInt(1000))

	// Create mix of successful and failing transactions
	successTxs := []string{"0x1111", "0x2222", "0x3333"}
	failTxs := []string{"0x4444", "0x5555"}

	// Add successful transactions
	for _, hashStr := range successTxs {
		txHash := common.HexToHash(hashStr)
		err := db.Create(&models.MessageWaitsEth{
			SignedTxHash: txHash.Hex(),
			TxStatus:     "pending",
		}).Error
		require.NoError(t, err)

		receipt := createTestReceipt(900, 1)
		tx := createTestTransaction(1)
		client.addReceipt(txHash, receipt, 0)
		client.addTransaction(txHash, tx, 0)
	}

	// Add failing transactions (permanent failures)
	for _, hashStr := range failTxs {
		txHash := common.HexToHash(hashStr)
		err := db.Create(&models.MessageWaitsEth{
			SignedTxHash: txHash.Hex(),
			TxStatus:     "pending",
		}).Error
		require.NoError(t, err)

		// These will fail with max retries
		receipt := createTestReceipt(900, 1)
		client.addReceipt(txHash, receipt, 10) // Will always fail
	}

	// Run update
	mw.update()

	// Verify only successful transactions were updated
	var confirmedCount int64
	err := db.Model(&models.MessageWaitsEth{}).
		Where("tx_status = ?", "confirmed").
		Count(&confirmedCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(len(successTxs)), confirmedCount)

	// Verify failed transactions remain pending
	var pendingCount int64
	err = db.Model(&models.MessageWaitsEth{}).
		Where("tx_status = ?", "pending").
		Count(&pendingCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(len(failTxs)), pendingCount)
}

func TestUpdate_NoBestBlockNumber(t *testing.T) {
	db := setupTestDB(t)
	client := newFakeEthClient()

	mw := &MessageWatcherEth{
		db:               db,
		api:              client,
		maxEthAPIRetries: 3,
	}

	// Don't set best block number

	// Create a pending transaction
	err := db.Create(&models.MessageWaitsEth{
		SignedTxHash: "0x123",
		TxStatus:     "pending",
	}).Error
	require.NoError(t, err)

	// Run update - should return early
	mw.update()

	// Verify no API calls were made
	assert.Equal(t, int32(0), client.getCallCount("TransactionReceipt"))
	assert.Equal(t, int32(0), client.getCallCount("TransactionByHash"))
}
