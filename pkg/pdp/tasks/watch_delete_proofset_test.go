package tasks

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/storacha/piri/pkg/database/gormdb"
	"github.com/storacha/piri/pkg/pdp/service/contract"
	"github.com/storacha/piri/pkg/pdp/service/models"
)

func TestExtractProofSetDeleteInfo_Success(t *testing.T) {
	t.Parallel()

	// Build a synthetic receipt containing the ProofSetDeleted event
	pdpABI, err := contract.PDPVerifierMetaData()
	require.NoError(t, err)

	ev, ok := pdpABI.Events["ProofSetDeleted"]
	require.True(t, ok, "expected ProofSetDeleted event in ABI")

	proofSetID := uint64(42)
	deletedLeafCount := uint64(7)

	// Topics: [event ID, indexed setId]
	topics := []common.Hash{
		ev.ID,
		common.BigToHash(new(big.Int).SetUint64(proofSetID)),
	}

	// Data: non-indexed parameters packed (deletedLeafCount)
	packed, err := ev.Inputs.NonIndexed().Pack(new(big.Int).SetUint64(deletedLeafCount))
	require.NoError(t, err)

	receipt := &types.Receipt{Logs: []*types.Log{{Topics: topics, Data: packed}}}

	gotSetID, gotDeleted, err := extractProofSetDeleteInfo(receipt)
	require.NoError(t, err)
	assert.Equal(t, proofSetID, gotSetID)
	assert.Equal(t, deletedLeafCount, gotDeleted)
}

func TestExtractProofSetDeleteInfo_NotFound(t *testing.T) {
	t.Parallel()

	// Receipt without the target event
	receipt := &types.Receipt{Logs: []*types.Log{}}
	_, _, err := extractProofSetDeleteInfo(receipt)
	require.Error(t, err)
}

func TestCleanupProofSet_RemovesRecords(t *testing.T) {
	t.Parallel()

	// Setup ephemeral DB
	db, err := gormdb.New(t.TempDir() + "/test.db")
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrateDB(t.Context(), db))

	// Insert a proof set and associated roots
	ps := &models.PDPProofSet{Service: "svc-a", CreateMessageHash: "0xabc"}
	require.NoError(t, db.Create(ps).Error)

	// Seed a message_waits_eth row referenced by roots' AddMessageHash
	rootTxHash := common.HexToHash("0x1111")
	require.NoError(t, db.Create(&models.MessageWaitsEth{
		SignedTxHash: rootTxHash.Hex(),
		TxStatus:     "confirmed",
	}).Error)

	r1 := &models.PDPProofsetRoot{ProofsetID: ps.ID, RootID: 1, AddMessageHash: rootTxHash.Hex(), Root: "r1", Subroot: "s1"}
	r2 := &models.PDPProofsetRoot{ProofsetID: ps.ID, RootID: 2, AddMessageHash: rootTxHash.Hex(), Root: "r2", Subroot: "s2"}
	require.NoError(t, db.Create(r1).Error)
	require.NoError(t, db.Create(r2).Error)

	// Call cleanup
	err = cleanupProofSet(t.Context(), db, uint64(ps.ID))
	require.NoError(t, err)

	// Verify deletions
	var count int64
	require.NoError(t, db.Model(&models.PDPProofsetRoot{}).Where("proofset_id = ?", ps.ID).Count(&count).Error)
	assert.Equal(t, int64(0), count)

	require.NoError(t, db.Model(&models.PDPProofSet{}).Where("id = ?", ps.ID).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func TestProcessProofSetDelete_UpdatesStatusAndCleansUp(t *testing.T) {
	t.Parallel()

	// Setup DB and seed required rows mimicking a confirmed tx in message_waits_eth
	db, err := gormdb.New(t.TempDir() + "/test.db")
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrateDB(t.Context(), db))

	// Create proof set to be deleted
	ps := &models.PDPProofSet{Service: "svc-a", CreateMessageHash: "0xabc"}
	require.NoError(t, db.Create(ps).Error)

	// We will reuse txHash below for both message_waits_eth and root AddMessageHash
	txHash := common.HexToHash("0xdeadbeef")

	// Seed message_waits_eth with the receipt JSON (added after we build it)
	// First create a placeholder row so the FK for root insert passes, we'll update later
	require.NoError(t, db.Create(&models.MessageWaitsEth{
		SignedTxHash: txHash.Hex(),
		TxStatus:     "pending",
	}).Error)

	require.NoError(t, db.Create(&models.PDPProofsetRoot{ProofsetID: ps.ID, RootID: 1, AddMessageHash: txHash.Hex(), Root: "r", Subroot: "s"}).Error)

	// Build a fake receipt JSON carrying ProofSetDeleted event
	pdpABI, err := contract.PDPVerifierMetaData()
	require.NoError(t, err)
	ev := pdpABI.Events["ProofSetDeleted"]

	topics := []common.Hash{ev.ID, common.BigToHash(new(big.Int).SetUint64(uint64(ps.ID)))}
	data, err := ev.Inputs.NonIndexed().Pack(new(big.Int).SetUint64(1))
	require.NoError(t, err)
	rec := &types.Receipt{Logs: []*types.Log{{Topics: topics, Data: data}}}
	recJSON, err := json.Marshal(rec)
	require.NoError(t, err)

	// Update message_waits_eth with the receipt JSON and status
	require.NoError(t, db.Model(&models.MessageWaitsEth{}).
		Where("signed_tx_hash = ?", txHash.Hex()).
		Updates(map[string]any{
			"tx_status":  "confirmed",
			"tx_success": models.Ptr(true),
			"tx_receipt": recJSON,
		}).Error)

	// Seed pdp_proofset_deletes row (ok=true, proofset_deleted=false)
	ok := true
	require.NoError(t, db.Create(&models.PDPProofsetDelete{
		DeleteMessageHash: txHash.Hex(),
		Ok:                &ok,
		ProofsetDeleted:   false,
		Service:           "svc-a",
	}).Error)

	// Run the processor
	err = processProofSetDelete(t.Context(), db, models.PDPProofsetDelete{
		DeleteMessageHash: txHash.Hex(),
		Service:           "svc-a",
	}, nil, nil)
	require.NoError(t, err)

	// Verify cleanup and status update
	var cnt int64
	require.NoError(t, db.Model(&models.PDPProofSet{}).Where("id = ?", ps.ID).Count(&cnt).Error)
	assert.Equal(t, int64(0), cnt)

	require.NoError(t, db.Model(&models.PDPProofsetDelete{}).Where("delete_message_hash = ? AND proofset_deleted = ?", txHash.Hex(), true).Count(&cnt).Error)
	assert.Equal(t, int64(1), cnt)
}
