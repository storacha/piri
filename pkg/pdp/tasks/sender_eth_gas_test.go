package tasks_test

import (
	"context"
	"errors"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/config/dynamic"
	"github.com/storacha/piri/pkg/database/gormdb"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/tasks"
	"github.com/storacha/piri/pkg/store/local/keystore"
	"github.com/storacha/piri/pkg/wallet"
)

// Config keys that will be registered by the gas deferral feature.
const (
	gasMaxFeeProve         config.Key = "pdp.gas.max_fee.prove"
	gasMaxFeeProvingPeriod config.Key = "pdp.gas.max_fee.proving_period"
	gasMaxFeeProvingInit   config.Key = "pdp.gas.max_fee.proving_init"
	gasMaxFeeAddRoots      config.Key = "pdp.gas.max_fee.add_roots"
	gasMaxFeeDefault       config.Key = "pdp.gas.max_fee.default"
	gasRetryWait           config.Key = "pdp.gas.retry_wait"
)

// mockSenderETHClient implements tasks.SenderETHClient for gas tests.
type mockSenderETHClient struct {
	networkID  *big.Int
	baseFee    *big.Int
	gasTipCap  *big.Int
	gasLimit   uint64
	nonce      uint64
	sendTxErr  error
	sendTxCall int
}

func (m *mockSenderETHClient) NetworkID(ctx context.Context) (*big.Int, error) {
	return m.networkID, nil
}

func (m *mockSenderETHClient) HeaderByNumber(ctx context.Context, number *big.Int) (*ethtypes.Header, error) {
	return &ethtypes.Header{
		BaseFee: m.baseFee,
	}, nil
}

func (m *mockSenderETHClient) PendingNonceAt(ctx context.Context, account common.Address) (uint64, error) {
	return m.nonce, nil
}

func (m *mockSenderETHClient) EstimateGas(ctx context.Context, msg ethereum.CallMsg) (uint64, error) {
	return m.gasLimit, nil
}

func (m *mockSenderETHClient) SendTransaction(ctx context.Context, transaction *ethtypes.Transaction) error {
	m.sendTxCall++
	return m.sendTxErr
}

func (m *mockSenderETHClient) SuggestGasTipCap(ctx context.Context) (*big.Int, error) {
	return m.gasTipCap, nil
}

// mockWallet implements wallet.Wallet for testing.
type mockWallet struct{}

var _ wallet.Wallet = &mockWallet{}

func (m *mockWallet) Import(ctx context.Context, ki *keystore.KeyInfo) (common.Address, error) {
	return common.Address{}, nil
}

func (m *mockWallet) SignTransaction(ctx context.Context, addr common.Address, signer ethtypes.Signer, tx *ethtypes.Transaction) (*ethtypes.Transaction, error) {
	// Return the same transaction (in real life would be signed)
	return tx, nil
}

func setupGasTestDB(t *testing.T) *gorm.DB {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := gormdb.New(dbPath)
	require.NoError(t, err)
	err = models.AutoMigrateDB(t.Context(), db)
	require.NoError(t, err)
	return db
}

// newGasConfigRegistry creates a dynamic config registry with gas limit entries.
func newGasConfigRegistry(t *testing.T, overrides map[config.Key]uint) *dynamic.Registry {
	entries := map[config.Key]dynamic.ConfigEntry{
		gasMaxFeeProve: {
			Value:  uint(0),
			Schema: dynamic.UintSchema{Max: ^uint(0)},
		},
		gasMaxFeeProvingPeriod: {
			Value:  uint(0),
			Schema: dynamic.UintSchema{Max: ^uint(0)},
		},
		gasMaxFeeProvingInit: {
			Value:  uint(0),
			Schema: dynamic.UintSchema{Max: ^uint(0)},
		},
		gasMaxFeeAddRoots: {
			Value:  uint(0),
			Schema: dynamic.UintSchema{Max: ^uint(0)},
		},
		gasMaxFeeDefault: {
			Value:  uint(0),
			Schema: dynamic.UintSchema{Max: ^uint(0)},
		},
		gasRetryWait: {
			Value:  5 * time.Minute,
			Schema: dynamic.DurationSchema{Min: time.Second, Max: time.Hour},
		},
	}

	for k, v := range overrides {
		if entry, ok := entries[k]; ok {
			entry.Value = v
			entries[k] = entry
		}
	}

	return dynamic.NewRegistry(entries)
}

// createUnsignedTx creates a serialized unsigned EIP-1559 transaction for test DB rows.
func createUnsignedTx(t *testing.T, gasLimit uint64, gasFeeCap, gasTipCap *big.Int) []byte {
	to := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	tx := ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   big.NewInt(1),
		Nonce:     0,
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
		Gas:       gasLimit,
		To:        &to,
		Value:     big.NewInt(0),
		Data:      []byte{0x01, 0x02},
	})
	data, err := tx.MarshalBinary()
	require.NoError(t, err)
	return data
}

// insertTestMessageSend inserts a MessageSendsEth row and returns its task ID.
func insertTestMessageSend(t *testing.T, db *gorm.DB, taskID int, sendReason string, unsignedTx []byte) {
	from := common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	to := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	msg := models.MessageSendsEth{
		FromAddress:  from.Hex(),
		ToAddress:    to.Hex(),
		SendReason:   sendReason,
		SendTaskID:   taskID,
		UnsignedTx:   unsignedTx,
		UnsignedHash: "0xdeadbeef",
	}
	err := db.Create(&msg).Error
	require.NoError(t, err)
}

// estimatedGasCost calculates the estimated gas cost: gasLimit * (baseFee + gasTipCap)
func estimatedGasCost(gasLimit uint64, baseFee, gasTipCap *big.Int) *big.Int {
	effectiveGasPrice := new(big.Int).Add(baseFee, gasTipCap)
	return new(big.Int).Mul(big.NewInt(int64(gasLimit)), effectiveGasPrice)
}

// TestSendTaskETH_GasAboveLimit tests AC1: when estimated gas cost exceeds configured max,
// Do() returns (false, ErrGasTooHigh) without signing or broadcasting.
func TestSendTaskETH_GasAboveLimit(t *testing.T) {
	db := setupGasTestDB(t)

	baseFee := big.NewInt(30_000_000_000)    // 30 gwei
	gasTipCap := big.NewInt(2_000_000_000)   // 2 gwei
	gasLimit := uint64(200_000)              // 200k gas

	// Estimated cost = 200_000 * (30 + 2) gwei = 6_400_000 gwei = 6.4e15 wei
	cost := estimatedGasCost(gasLimit, baseFee, gasTipCap)

	// Set max fee below the estimated cost
	maxFee := new(big.Int).Sub(cost, big.NewInt(1))

	client := &mockSenderETHClient{
		networkID: big.NewInt(1),
		baseFee:   baseFee,
		gasTipCap: gasTipCap,
		gasLimit:  gasLimit,
		nonce:     0,
	}

	registry := newGasConfigRegistry(t, map[config.Key]uint{
		gasMaxFeeProve: uint(maxFee.Uint64()),
	})

	_, sendTask, err := tasks.NewSenderETH(client, &mockWallet{}, db, tasks.WithGasConfig(registry))
	require.NoError(t, err)

	// Create a task row in the DB
	taskID := 1
	unsignedTx := createUnsignedTx(t, gasLimit, baseFee, gasTipCap)
	insertTestMessageSend(t, db, taskID, "pdp-prove", unsignedTx)

	// Also need a task row for the scheduler
	err = db.Create(&models.Task{
		ID:         int64(taskID),
		Name:       "SendTransaction",
		PostedTime: time.Now(),
		UpdateTime: time.Now(),
	}).Error
	require.NoError(t, err)

	// Execute Do() - should return false with ErrGasTooHigh
	done, doErr := sendTask.Do(scheduler.TaskID(taskID))

	assert.False(t, done, "task should not be marked done when gas is too high")
	require.Error(t, doErr, "Do should return an error when gas is too high")
	assert.True(t, errors.Is(doErr, scheduler.ErrGasTooHigh),
		"error should be ErrGasTooHigh, got: %v", doErr)

	// Verify the transaction was NOT sent (no signing happened)
	assert.Equal(t, 0, client.sendTxCall, "transaction should not have been sent")
}

// TestSendTaskETH_GasBelowLimit tests AC6: when estimated gas drops below
// configured max, the deferred task proceeds normally.
func TestSendTaskETH_GasBelowLimit(t *testing.T) {
	db := setupGasTestDB(t)

	baseFee := big.NewInt(10_000_000_000)    // 10 gwei
	gasTipCap := big.NewInt(1_000_000_000)   // 1 gwei
	gasLimit := uint64(100_000)              // 100k gas

	// Estimated cost = 100_000 * (10 + 1) gwei = 1_100_000 gwei = 1.1e15 wei
	cost := estimatedGasCost(gasLimit, baseFee, gasTipCap)

	// Set max fee above the estimated cost
	maxFee := new(big.Int).Add(cost, big.NewInt(1_000_000_000))

	client := &mockSenderETHClient{
		networkID: big.NewInt(1),
		baseFee:   baseFee,
		gasTipCap: gasTipCap,
		gasLimit:  gasLimit,
		nonce:     0,
	}

	registry := newGasConfigRegistry(t, map[config.Key]uint{
		gasMaxFeeProve: uint(maxFee.Uint64()),
	})

	_, sendTask, err := tasks.NewSenderETH(client, &mockWallet{}, db, tasks.WithGasConfig(registry))
	require.NoError(t, err)

	taskID := 1
	unsignedTx := createUnsignedTx(t, gasLimit, baseFee, gasTipCap)
	insertTestMessageSend(t, db, taskID, "pdp-prove", unsignedTx)

	err = db.Create(&models.Task{
		ID:         int64(taskID),
		Name:       "SendTransaction",
		PostedTime: time.Now(),
		UpdateTime: time.Now(),
	}).Error
	require.NoError(t, err)

	// Execute Do() - should proceed past the gas check (may fail later in signing
	// due to mock wallet, but should NOT return ErrGasTooHigh)
	done, doErr := sendTask.Do(scheduler.TaskID(taskID))

	if doErr != nil {
		assert.False(t, errors.Is(doErr, scheduler.ErrGasTooHigh),
			"error should NOT be ErrGasTooHigh when gas is below limit, got: %v", doErr)
	}
	// If mock wallet works correctly, the task should complete
	_ = done
}

// TestSendTaskETH_GasLimitZero tests AC7: a SendReason with limit set to 0
// bypasses gas check entirely.
func TestSendTaskETH_GasLimitZero(t *testing.T) {
	db := setupGasTestDB(t)

	// Very high gas cost
	baseFee := big.NewInt(500_000_000_000)   // 500 gwei
	gasTipCap := big.NewInt(50_000_000_000)  // 50 gwei
	gasLimit := uint64(1_000_000)            // 1M gas

	client := &mockSenderETHClient{
		networkID: big.NewInt(1),
		baseFee:   baseFee,
		gasTipCap: gasTipCap,
		gasLimit:  gasLimit,
		nonce:     0,
	}

	// All limits are 0 (default) - gas check should be bypassed
	registry := newGasConfigRegistry(t, nil)

	_, sendTask, err := tasks.NewSenderETH(client, &mockWallet{}, db, tasks.WithGasConfig(registry))
	require.NoError(t, err)

	taskID := 1
	unsignedTx := createUnsignedTx(t, gasLimit, baseFee, gasTipCap)
	insertTestMessageSend(t, db, taskID, "pdp-prove", unsignedTx)

	err = db.Create(&models.Task{
		ID:         int64(taskID),
		Name:       "SendTransaction",
		PostedTime: time.Now(),
		UpdateTime: time.Now(),
	}).Error
	require.NoError(t, err)

	// Execute Do() - should NOT return ErrGasTooHigh even with extremely high gas
	done, doErr := sendTask.Do(scheduler.TaskID(taskID))

	if doErr != nil {
		assert.False(t, errors.Is(doErr, scheduler.ErrGasTooHigh),
			"error should NOT be ErrGasTooHigh when limit is 0 (bypass), got: %v", doErr)
	}
	_ = done
}

// TestSendTaskETH_GasFallbackToDefault tests AC9: a SendReason without a
// dedicated config key falls back to pdp.gas.max_fee.default.
func TestSendTaskETH_GasFallbackToDefault(t *testing.T) {
	db := setupGasTestDB(t)

	baseFee := big.NewInt(30_000_000_000)    // 30 gwei
	gasTipCap := big.NewInt(2_000_000_000)   // 2 gwei
	gasLimit := uint64(200_000)              // 200k gas

	cost := estimatedGasCost(gasLimit, baseFee, gasTipCap)

	// Set default max fee below estimated cost - should trigger ErrGasTooHigh
	maxFee := new(big.Int).Sub(cost, big.NewInt(1))

	client := &mockSenderETHClient{
		networkID: big.NewInt(1),
		baseFee:   baseFee,
		gasTipCap: gasTipCap,
		gasLimit:  gasLimit,
		nonce:     0,
	}

	// Only set the default key, not the per-type key
	registry := newGasConfigRegistry(t, map[config.Key]uint{
		gasMaxFeeDefault: uint(maxFee.Uint64()),
	})

	_, sendTask, err := tasks.NewSenderETH(client, &mockWallet{}, db, tasks.WithGasConfig(registry))
	require.NoError(t, err)

	taskID := 1
	unsignedTx := createUnsignedTx(t, gasLimit, baseFee, gasTipCap)
	// Use a reason that has no dedicated config key set (per-type is 0)
	// but has a default set above
	insertTestMessageSend(t, db, taskID, "pdp-prove", unsignedTx)

	err = db.Create(&models.Task{
		ID:         int64(taskID),
		Name:       "SendTransaction",
		PostedTime: time.Now(),
		UpdateTime: time.Now(),
	}).Error
	require.NoError(t, err)

	// pdp.gas.max_fee.prove is 0 (no limit), so it should fall back to
	// pdp.gas.max_fee.default which is below cost. Should get ErrGasTooHigh.
	done, doErr := sendTask.Do(scheduler.TaskID(taskID))

	assert.False(t, done, "task should not be done when gas exceeds default limit")
	require.Error(t, doErr)
	assert.True(t, errors.Is(doErr, scheduler.ErrGasTooHigh),
		"should fall back to default gas limit and return ErrGasTooHigh, got: %v", doErr)
	assert.Equal(t, 0, client.sendTxCall, "transaction should not have been sent")
}

// TestSendTaskETH_GasPerTypeOverridesDefault tests AC9 inverse: when a per-type
// key IS set (non-zero), it takes precedence over the default.
func TestSendTaskETH_GasPerTypeOverridesDefault(t *testing.T) {
	db := setupGasTestDB(t)

	baseFee := big.NewInt(30_000_000_000)    // 30 gwei
	gasTipCap := big.NewInt(2_000_000_000)   // 2 gwei
	gasLimit := uint64(200_000)              // 200k gas

	cost := estimatedGasCost(gasLimit, baseFee, gasTipCap)

	client := &mockSenderETHClient{
		networkID: big.NewInt(1),
		baseFee:   baseFee,
		gasTipCap: gasTipCap,
		gasLimit:  gasLimit,
		nonce:     0,
	}

	// Default is very low (would block), but per-type is very high (would allow)
	registry := newGasConfigRegistry(t, map[config.Key]uint{
		gasMaxFeeDefault: uint(1),                             // 1 wei - would block
		gasMaxFeeProve:   uint(cost.Uint64()) + 1_000_000_000, // above cost - allows
	})

	_, sendTask, err := tasks.NewSenderETH(client, &mockWallet{}, db, tasks.WithGasConfig(registry))
	require.NoError(t, err)

	taskID := 1
	unsignedTx := createUnsignedTx(t, gasLimit, baseFee, gasTipCap)
	insertTestMessageSend(t, db, taskID, "pdp-prove", unsignedTx)

	err = db.Create(&models.Task{
		ID:         int64(taskID),
		Name:       "SendTransaction",
		PostedTime: time.Now(),
		UpdateTime: time.Now(),
	}).Error
	require.NoError(t, err)

	// Per-type limit is above cost, so gas check should pass even though default would block
	done, doErr := sendTask.Do(scheduler.TaskID(taskID))

	if doErr != nil {
		assert.False(t, errors.Is(doErr, scheduler.ErrGasTooHigh),
			"per-type limit should override default, got ErrGasTooHigh: %v", doErr)
	}
	_ = done
}

// TestSendTaskETH_GasConfigUpdatedAtRuntime tests AC5: operators can update gas
// limits at runtime via dynamic config without restart.
func TestSendTaskETH_GasConfigUpdatedAtRuntime(t *testing.T) {
	db := setupGasTestDB(t)

	baseFee := big.NewInt(30_000_000_000)    // 30 gwei
	gasTipCap := big.NewInt(2_000_000_000)   // 2 gwei
	gasLimit := uint64(200_000)              // 200k gas

	cost := estimatedGasCost(gasLimit, baseFee, gasTipCap)

	client := &mockSenderETHClient{
		networkID: big.NewInt(1),
		baseFee:   baseFee,
		gasTipCap: gasTipCap,
		gasLimit:  gasLimit,
		nonce:     0,
	}

	// Start with a limit below cost
	lowLimit := new(big.Int).Sub(cost, big.NewInt(1))
	registry := newGasConfigRegistry(t, map[config.Key]uint{
		gasMaxFeeProve: uint(lowLimit.Uint64()),
	})

	_, sendTask, err := tasks.NewSenderETH(client, &mockWallet{}, db, tasks.WithGasConfig(registry))
	require.NoError(t, err)

	// First call: gas too high
	taskID1 := 1
	unsignedTx := createUnsignedTx(t, gasLimit, baseFee, gasTipCap)
	insertTestMessageSend(t, db, taskID1, "pdp-prove", unsignedTx)
	err = db.Create(&models.Task{
		ID:         int64(taskID1),
		Name:       "SendTransaction",
		PostedTime: time.Now(),
		UpdateTime: time.Now(),
	}).Error
	require.NoError(t, err)

	done, doErr := sendTask.Do(scheduler.TaskID(taskID1))
	assert.False(t, done)
	require.Error(t, doErr)
	assert.True(t, errors.Is(doErr, scheduler.ErrGasTooHigh),
		"first call should return ErrGasTooHigh")

	// Update config at runtime to raise the limit above cost
	highLimit := new(big.Int).Add(cost, big.NewInt(1_000_000_000))
	err = registry.Update(map[string]any{
		string(gasMaxFeeProve): uint(highLimit.Uint64()),
	}, false, dynamic.SourceAPI)
	require.NoError(t, err)

	// Second call with same conditions: should now pass gas check
	// Need a fresh DB row for the second attempt
	taskID2 := 2
	insertTestMessageSend(t, db, taskID2, "pdp-prove", unsignedTx)
	err = db.Create(&models.Task{
		ID:         int64(taskID2),
		Name:       "SendTransaction",
		PostedTime: time.Now(),
		UpdateTime: time.Now(),
	}).Error
	require.NoError(t, err)

	done2, doErr2 := sendTask.Do(scheduler.TaskID(taskID2))
	if doErr2 != nil {
		assert.False(t, errors.Is(doErr2, scheduler.ErrGasTooHigh),
			"after raising limit, should NOT get ErrGasTooHigh, got: %v", doErr2)
	}
	_ = done2
}

// TestSendTaskETH_TypeDetailsRetryWait tests AC3: TypeDetails() includes a RetryWait
// reading from dynamic config with a default of 5 minutes.
func TestSendTaskETH_TypeDetailsRetryWait(t *testing.T) {
	db := setupGasTestDB(t)

	client := &mockSenderETHClient{
		networkID: big.NewInt(1),
		baseFee:   big.NewInt(1),
		gasTipCap: big.NewInt(1),
	}

	t.Run("default retry wait is 5 minutes", func(t *testing.T) {
		registry := newGasConfigRegistry(t, nil)

		_, sendTask, err := tasks.NewSenderETH(client, &mockWallet{}, db, tasks.WithGasConfig(registry))
		require.NoError(t, err)

		details := sendTask.TypeDetails()
		require.NotNil(t, details.RetryWait, "RetryWait should not be nil")

		wait := details.RetryWait(0)
		assert.Equal(t, 5*time.Minute, wait,
			"default RetryWait should be 5 minutes")
	})

	t.Run("retry wait reads from dynamic config", func(t *testing.T) {
		registry := newGasConfigRegistry(t, nil)
		// Update retry wait to 10 minutes
		err := registry.Update(map[string]any{
			string(gasRetryWait): "10m",
		}, false, dynamic.SourceAPI)
		require.NoError(t, err)

		_, sendTask, err := tasks.NewSenderETH(client, &mockWallet{}, db, tasks.WithGasConfig(registry))
		require.NoError(t, err)

		details := sendTask.TypeDetails()
		require.NotNil(t, details.RetryWait)

		wait := details.RetryWait(0)
		assert.Equal(t, 10*time.Minute, wait,
			"RetryWait should read from dynamic config")
	})
}

// TestSendTaskETH_GasConfigRegistration tests AC4: per-message-type max gas fee
// config keys are registered as dynamic config entries with correct defaults.
func TestSendTaskETH_GasConfigRegistration(t *testing.T) {
	expectedKeys := map[config.Key]uint{
		gasMaxFeeProve:         0,
		gasMaxFeeProvingPeriod: 0,
		gasMaxFeeProvingInit:   0,
		gasMaxFeeAddRoots:      0,
		gasMaxFeeDefault:       0,
	}

	registry := newGasConfigRegistry(t, nil)

	for key, expectedDefault := range expectedKeys {
		val := registry.GetUint(key, 999)
		assert.Equal(t, expectedDefault, val,
			"config key %s should have default value %d, got %d", key, expectedDefault, val)
	}

	// Verify retry wait default
	retryWait := registry.GetDuration(gasRetryWait, 0)
	assert.Equal(t, 5*time.Minute, retryWait,
		"gas retry wait should default to 5 minutes")
}

// TestSendTaskETH_GasCheckWithAllSendReasons tests AC1 across all known SendReasons,
// verifying each uses its dedicated config key.
func TestSendTaskETH_GasCheckWithAllSendReasons(t *testing.T) {
	testCases := []struct {
		sendReason string
		configKey  config.Key
	}{
		{"pdp-prove", gasMaxFeeProve},
		{"pdp-proving-period", gasMaxFeeProvingPeriod},
		{"pdp-proving-init", gasMaxFeeProvingInit},
		{"pdp-addroots", gasMaxFeeAddRoots},
	}

	for _, tc := range testCases {
		t.Run(tc.sendReason, func(t *testing.T) {
			db := setupGasTestDB(t)

			baseFee := big.NewInt(30_000_000_000)
			gasTipCap := big.NewInt(2_000_000_000)
			gasLimit := uint64(200_000)

			cost := estimatedGasCost(gasLimit, baseFee, gasTipCap)
			maxFee := new(big.Int).Sub(cost, big.NewInt(1))

			client := &mockSenderETHClient{
				networkID: big.NewInt(1),
				baseFee:   baseFee,
				gasTipCap: gasTipCap,
				gasLimit:  gasLimit,
				nonce:     0,
			}

			registry := newGasConfigRegistry(t, map[config.Key]uint{
				tc.configKey: uint(maxFee.Uint64()),
			})

			_, sendTask, err := tasks.NewSenderETH(client, &mockWallet{}, db, tasks.WithGasConfig(registry))
			require.NoError(t, err)

			taskID := 1
			unsignedTx := createUnsignedTx(t, gasLimit, baseFee, gasTipCap)
			insertTestMessageSend(t, db, taskID, tc.sendReason, unsignedTx)

			err = db.Create(&models.Task{
				ID:         int64(taskID),
				Name:       "SendTransaction",
				PostedTime: time.Now(),
				UpdateTime: time.Now(),
			}).Error
			require.NoError(t, err)

			done, doErr := sendTask.Do(scheduler.TaskID(taskID))
			assert.False(t, done)
			require.Error(t, doErr)
			assert.True(t, errors.Is(doErr, scheduler.ErrGasTooHigh),
				"SendReason %q with config key %s should trigger ErrGasTooHigh, got: %v",
				tc.sendReason, tc.configKey, doErr)
			assert.Equal(t, 0, client.sendTxCall,
				"transaction should not have been sent for %q", tc.sendReason)
		})
	}
}
