package service

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
)

// mockActivePiecesProvider implements ActivePiecesProvider for testing
type mockActivePiecesProvider struct {
	pieces    map[uint64]*smartcontracts.ActivePieces // keyed by offset
	count     *big.Int
	countErr  error
	piecesErr error
}

func (m *mockActivePiecesProvider) GetActivePieceCount(ctx context.Context, setId *big.Int) (*big.Int, error) {
	if m.countErr != nil {
		return nil, m.countErr
	}
	return m.count, nil
}

func (m *mockActivePiecesProvider) GetActivePieces(ctx context.Context, setID *big.Int, offset *big.Int, limit *big.Int) (*smartcontracts.ActivePieces, error) {
	if m.piecesErr != nil {
		return nil, m.piecesErr
	}
	result, ok := m.pieces[offset.Uint64()]
	if !ok {
		return &smartcontracts.ActivePieces{HasMore: false}, nil
	}
	return result, nil
}

// setupRepairTestDB creates an in-memory SQLite database for repair tests
func setupRepairTestDB(t *testing.T) *gorm.DB {
	dbName := fmt.Sprintf("file:repair-test-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)

	sqlDb, err := db.DB()
	require.NoError(t, err)
	sqlDb.SetMaxOpenConns(1)

	// Migrate the models needed for repair tests
	// Note: We need to create tables in order to respect foreign key constraints
	// First create message_waits_eth, then pdp_proof_sets, then the dependent tables
	err = db.AutoMigrate(
		&models.MessageWaitsEth{},
		&models.PDPProofSet{},
		&models.PDPProofsetRoot{},
		&models.PDPProofsetRootAdd{},
	)
	require.NoError(t, err)
	return db
}

// testCID creates a test CID from the given data string
func testCID(t *testing.T, data string) cid.Cid {
	h, err := multihash.Sum([]byte(data), multihash.SHA2_256, -1)
	require.NoError(t, err)
	return cid.NewCidV1(cid.Raw, h)
}

func TestRepairProofSetCore_NoRepairNeeded(t *testing.T) {
	ctx := context.Background()
	db := setupRepairTestDB(t)
	proofSetID := uint64(1)

	// Setup: Create message wait first (for foreign key)
	msgWait := models.MessageWaitsEth{SignedTxHash: "0x123", TxStatus: "confirmed"}
	require.NoError(t, db.Create(&msgWait).Error)

	// Setup: Create proof set
	proofSet := models.PDPProofSet{ID: int64(proofSetID), CreateMessageHash: "0x123", Service: "test"}
	require.NoError(t, db.Create(&proofSet).Error)

	// Setup: Create root in DB
	rootCID := testCID(t, "root1")
	subrootCID := testCID(t, "subroot1")
	dbRoot := models.PDPProofsetRoot{
		ProofsetID:     int64(proofSetID),
		RootID:         0,
		SubrootOffset:  0,
		Root:           rootCID.String(),
		AddMessageHash: "0x123",
		Subroot:        subrootCID.String(),
		SubrootSize:    1024,
	}
	require.NoError(t, db.Create(&dbRoot).Error)

	// Setup: Mock returns same root as on-chain
	mock := &mockActivePiecesProvider{
		count: big.NewInt(1),
		pieces: map[uint64]*smartcontracts.ActivePieces{
			0: {
				Pieces:   []cid.Cid{rootCID},
				PieceIds: []*big.Int{big.NewInt(0)},
				HasMore:  false,
			},
		},
	}

	// Execute
	result, err := repairProofSetCore(ctx, db, mock, proofSetID)

	// Assert
	require.NoError(t, err)
	require.Equal(t, 1, result.TotalOnChain)
	require.Equal(t, 1, result.TotalInDB)
	require.Equal(t, 0, result.TotalRepaired)
	require.Equal(t, 0, result.TotalUnrepaired)
}

func TestRepairProofSetCore_RepairsGap(t *testing.T) {
	ctx := context.Background()
	db := setupRepairTestDB(t)
	proofSetID := uint64(1)

	// Setup: Create message wait (pending - simulates Lotus lost state)
	msgWait := models.MessageWaitsEth{SignedTxHash: "0x456", TxStatus: "pending"}
	require.NoError(t, db.Create(&msgWait).Error)

	// Setup: Create proof set
	proofSet := models.PDPProofSet{ID: int64(proofSetID), CreateMessageHash: "0x456", Service: "test"}
	require.NoError(t, db.Create(&proofSet).Error)

	// Setup: Root exists on-chain but NOT in pdp_proofset_roots
	rootCID := testCID(t, "missing-root")
	subrootCID := testCID(t, "missing-subroot")
	pieceID := uint64(42)

	// Setup: Metadata exists in pdp_proofset_root_adds (stuck state)
	addMsgIndex := int64(0)
	rootAdd := models.PDPProofsetRootAdd{
		ProofsetID:      int64(proofSetID),
		AddMessageHash:  "0x456",
		SubrootOffset:   0,
		Root:            rootCID.String(),
		AddMessageOK:    nil, // NULL - simulates stuck state
		AddMessageIndex: &addMsgIndex,
		Subroot:         subrootCID.String(),
		SubrootSize:     2048,
	}
	require.NoError(t, db.Create(&rootAdd).Error)

	// Setup: Mock returns this root as active on-chain
	mock := &mockActivePiecesProvider{
		count: big.NewInt(1),
		pieces: map[uint64]*smartcontracts.ActivePieces{
			0: {
				Pieces:   []cid.Cid{rootCID},
				PieceIds: []*big.Int{big.NewInt(int64(pieceID))},
				HasMore:  false,
			},
		},
	}

	// Execute
	result, err := repairProofSetCore(ctx, db, mock, proofSetID)

	// Assert repair result
	require.NoError(t, err)
	require.Equal(t, 1, result.TotalOnChain)
	require.Equal(t, 0, result.TotalInDB) // Was empty before repair
	require.Equal(t, 1, result.TotalRepaired)
	require.Equal(t, 0, result.TotalUnrepaired)
	require.Len(t, result.RepairedEntries, 1)
	require.Equal(t, rootCID.String(), result.RepairedEntries[0].RootCID)
	require.Equal(t, pieceID, result.RepairedEntries[0].RootID)

	// Assert database state: root now in pdp_proofset_roots
	var roots []models.PDPProofsetRoot
	require.NoError(t, db.Where("proofset_id = ?", proofSetID).Find(&roots).Error)
	require.Len(t, roots, 1)
	require.Equal(t, int64(pieceID), roots[0].RootID)

	// Assert database state: entry removed from pdp_proofset_root_adds
	var adds []models.PDPProofsetRootAdd
	require.NoError(t, db.Where("proofset_id = ?", proofSetID).Find(&adds).Error)
	require.Len(t, adds, 0)

	// Assert database state: message_waits_eth marked as confirmed
	var updatedMsg models.MessageWaitsEth
	require.NoError(t, db.Where("signed_tx_hash = ?", "0x456").First(&updatedMsg).Error)
	require.Equal(t, "confirmed", updatedMsg.TxStatus)
	require.NotNil(t, updatedMsg.TxSuccess)
	require.True(t, *updatedMsg.TxSuccess)
}

func TestRepairProofSetCore_UnrepairableRoot(t *testing.T) {
	ctx := context.Background()
	db := setupRepairTestDB(t)
	proofSetID := uint64(1)

	// Setup: Create message wait for proof set
	msgWait := models.MessageWaitsEth{SignedTxHash: "0xabc", TxStatus: "confirmed"}
	require.NoError(t, db.Create(&msgWait).Error)

	// Setup: Create proof set
	proofSet := models.PDPProofSet{ID: int64(proofSetID), CreateMessageHash: "0xabc", Service: "test"}
	require.NoError(t, db.Create(&proofSet).Error)

	// Setup: Root exists on-chain but NO metadata in pdp_proofset_root_adds
	rootCID := testCID(t, "orphan-root")
	pieceID := uint64(99)

	mock := &mockActivePiecesProvider{
		count: big.NewInt(1),
		pieces: map[uint64]*smartcontracts.ActivePieces{
			0: {
				Pieces:   []cid.Cid{rootCID},
				PieceIds: []*big.Int{big.NewInt(int64(pieceID))},
				HasMore:  false,
			},
		},
	}

	// Execute
	result, err := repairProofSetCore(ctx, db, mock, proofSetID)

	// Assert
	require.NoError(t, err)
	require.Equal(t, 1, result.TotalOnChain)
	require.Equal(t, 0, result.TotalInDB)
	require.Equal(t, 0, result.TotalRepaired)
	require.Equal(t, 1, result.TotalUnrepaired)
	require.Len(t, result.UnrepairedEntries, 1)
	require.Equal(t, rootCID.String(), result.UnrepairedEntries[0].RootCID)
	require.Contains(t, result.UnrepairedEntries[0].Reason, "metadata not found")
}

func TestRepairProofSetCore_Pagination(t *testing.T) {
	ctx := context.Background()
	db := setupRepairTestDB(t)
	proofSetID := uint64(1)

	// Setup: Create message wait for proof set
	msgWait := models.MessageWaitsEth{SignedTxHash: "0x999", TxStatus: "confirmed"}
	require.NoError(t, db.Create(&msgWait).Error)

	// Setup: Create proof set
	proofSet := models.PDPProofSet{ID: int64(proofSetID), CreateMessageHash: "0x999", Service: "test"}
	require.NoError(t, db.Create(&proofSet).Error)

	// Setup: 150 roots on-chain (requires pagination with limit=100)
	rootCIDs := make([]cid.Cid, 150)
	pieceIds := make([]*big.Int, 150)
	for i := 0; i < 150; i++ {
		rootCIDs[i] = testCID(t, fmt.Sprintf("root-%d", i))
		pieceIds[i] = big.NewInt(int64(i))
	}

	// All roots also exist in DB (no repair needed, just testing pagination)
	for i := 0; i < 150; i++ {
		root := models.PDPProofsetRoot{
			ProofsetID:     int64(proofSetID),
			RootID:         int64(i),
			SubrootOffset:  0,
			Root:           rootCIDs[i].String(),
			AddMessageHash: "0x999",
			Subroot:        testCID(t, fmt.Sprintf("subroot-%d", i)).String(),
			SubrootSize:    1024,
		}
		require.NoError(t, db.Create(&root).Error)
	}

	// Mock returns paginated results
	mock := &mockActivePiecesProvider{
		count: big.NewInt(150),
		pieces: map[uint64]*smartcontracts.ActivePieces{
			0: {
				Pieces:   rootCIDs[:100],
				PieceIds: pieceIds[:100],
				HasMore:  true,
			},
			100: {
				Pieces:   rootCIDs[100:],
				PieceIds: pieceIds[100:],
				HasMore:  false,
			},
		},
	}

	// Execute
	result, err := repairProofSetCore(ctx, db, mock, proofSetID)

	// Assert pagination worked correctly
	require.NoError(t, err)
	require.Equal(t, 150, result.TotalOnChain)
	require.Equal(t, 150, result.TotalInDB)
	require.Equal(t, 0, result.TotalRepaired)
}

func TestRepairProofSetCore_ProofSetNotFound(t *testing.T) {
	ctx := context.Background()
	db := setupRepairTestDB(t)

	mock := &mockActivePiecesProvider{}

	// Execute with non-existent proof set
	result, err := repairProofSetCore(ctx, db, mock, 999)

	// Assert
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "not found")
}

func TestRepairProofSetCore_ChainError(t *testing.T) {
	ctx := context.Background()
	db := setupRepairTestDB(t)
	proofSetID := uint64(1)

	// Setup: Create message wait for proof set
	msgWait := models.MessageWaitsEth{SignedTxHash: "0xdef", TxStatus: "confirmed"}
	require.NoError(t, db.Create(&msgWait).Error)

	// Setup: Create proof set
	proofSet := models.PDPProofSet{ID: int64(proofSetID), CreateMessageHash: "0xdef", Service: "test"}
	require.NoError(t, db.Create(&proofSet).Error)

	mock := &mockActivePiecesProvider{
		countErr: fmt.Errorf("chain connection failed"),
	}

	// Execute
	result, err := repairProofSetCore(ctx, db, mock, proofSetID)

	// Assert
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "chain connection failed")
}

func TestRepairProofSetCore_MultipleSubroots(t *testing.T) {
	ctx := context.Background()
	db := setupRepairTestDB(t)
	proofSetID := uint64(1)

	// Setup: Create message wait
	msgWait := models.MessageWaitsEth{SignedTxHash: "0x789", TxStatus: "pending"}
	require.NoError(t, db.Create(&msgWait).Error)

	// Setup: Create proof set
	proofSet := models.PDPProofSet{ID: int64(proofSetID), CreateMessageHash: "0x789", Service: "test"}
	require.NoError(t, db.Create(&proofSet).Error)

	// Setup: One root with multiple subroots in pdp_proofset_root_adds
	rootCID := testCID(t, "multi-subroot-root")
	pieceID := uint64(50)
	addMsgIndex := int64(0)

	for i := 0; i < 3; i++ {
		rootAdd := models.PDPProofsetRootAdd{
			ProofsetID:      int64(proofSetID),
			AddMessageHash:  "0x789",
			SubrootOffset:   int64(i * 1024),
			Root:            rootCID.String(),
			AddMessageOK:    nil,
			AddMessageIndex: &addMsgIndex,
			Subroot:         testCID(t, fmt.Sprintf("subroot-%d", i)).String(),
			SubrootSize:     1024,
		}
		require.NoError(t, db.Create(&rootAdd).Error)
	}

	mock := &mockActivePiecesProvider{
		count: big.NewInt(1),
		pieces: map[uint64]*smartcontracts.ActivePieces{
			0: {
				Pieces:   []cid.Cid{rootCID},
				PieceIds: []*big.Int{big.NewInt(int64(pieceID))},
				HasMore:  false,
			},
		},
	}

	// Execute
	result, err := repairProofSetCore(ctx, db, mock, proofSetID)

	// Assert
	require.NoError(t, err)
	require.Equal(t, 1, result.TotalRepaired)
	require.Equal(t, 3, result.RepairedEntries[0].Subroots)

	// Assert all 3 subroots were created in pdp_proofset_roots
	var roots []models.PDPProofsetRoot
	require.NoError(t, db.Where("proofset_id = ? AND root_id = ?", proofSetID, pieceID).Find(&roots).Error)
	require.Len(t, roots, 3)

	// Assert MessageWaitsEth: single message hash updated (shared by all 3 subroots)
	var updatedMsg models.MessageWaitsEth
	require.NoError(t, db.Where("signed_tx_hash = ?", "0x789").First(&updatedMsg).Error)
	require.Equal(t, "confirmed", updatedMsg.TxStatus)
	require.NotNil(t, updatedMsg.TxSuccess)
	require.True(t, *updatedMsg.TxSuccess)
}

func TestRepairProofSetCore_MixedResults(t *testing.T) {
	ctx := context.Background()
	db := setupRepairTestDB(t)
	proofSetID := uint64(1)

	// Setup: Create message waits
	msgWait1 := models.MessageWaitsEth{SignedTxHash: "0xaaa", TxStatus: "pending"}
	require.NoError(t, db.Create(&msgWait1).Error)
	msgWait2 := models.MessageWaitsEth{SignedTxHash: "0xbbb", TxStatus: "confirmed"}
	require.NoError(t, db.Create(&msgWait2).Error)

	// Setup: Create proof set
	proofSet := models.PDPProofSet{ID: int64(proofSetID), CreateMessageHash: "0xaaa", Service: "test"}
	require.NoError(t, db.Create(&proofSet).Error)

	// Setup: One repairable root, one unrepairable root, one existing root
	repairableCID := testCID(t, "repairable-root")
	unrepairableCID := testCID(t, "unrepairable-root")
	existingCID := testCID(t, "existing-root")

	// Add metadata for repairable root
	addMsgIndex := int64(0)
	rootAdd := models.PDPProofsetRootAdd{
		ProofsetID:      int64(proofSetID),
		AddMessageHash:  "0xaaa",
		SubrootOffset:   0,
		Root:            repairableCID.String(),
		AddMessageOK:    nil,
		AddMessageIndex: &addMsgIndex,
		Subroot:         testCID(t, "repairable-subroot").String(),
		SubrootSize:     1024,
	}
	require.NoError(t, db.Create(&rootAdd).Error)

	// Add existing root to DB
	existingRoot := models.PDPProofsetRoot{
		ProofsetID:     int64(proofSetID),
		RootID:         100,
		SubrootOffset:  0,
		Root:           existingCID.String(),
		AddMessageHash: "0xbbb",
		Subroot:        testCID(t, "existing-subroot").String(),
		SubrootSize:    1024,
	}
	require.NoError(t, db.Create(&existingRoot).Error)

	// Mock returns all three roots as active on-chain
	mock := &mockActivePiecesProvider{
		count: big.NewInt(3),
		pieces: map[uint64]*smartcontracts.ActivePieces{
			0: {
				Pieces:   []cid.Cid{repairableCID, unrepairableCID, existingCID},
				PieceIds: []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(100)},
				HasMore:  false,
			},
		},
	}

	// Execute
	result, err := repairProofSetCore(ctx, db, mock, proofSetID)

	// Assert
	require.NoError(t, err)
	require.Equal(t, 3, result.TotalOnChain)
	require.Equal(t, 1, result.TotalInDB) // Only existing root was in DB
	require.Equal(t, 1, result.TotalRepaired)
	require.Equal(t, 1, result.TotalUnrepaired)

	// Verify repaired entry
	require.Len(t, result.RepairedEntries, 1)
	require.Equal(t, repairableCID.String(), result.RepairedEntries[0].RootCID)

	// Verify unrepairable entry
	require.Len(t, result.UnrepairedEntries, 1)
	require.Equal(t, unrepairableCID.String(), result.UnrepairedEntries[0].RootCID)

	// Assert MessageWaitsEth: pending message for repaired root is now confirmed
	var updatedMsg models.MessageWaitsEth
	require.NoError(t, db.Where("signed_tx_hash = ?", "0xaaa").First(&updatedMsg).Error)
	require.Equal(t, "confirmed", updatedMsg.TxStatus)
	require.NotNil(t, updatedMsg.TxSuccess)
	require.True(t, *updatedMsg.TxSuccess)

	// Assert MessageWaitsEth: already-confirmed message for existing root is unchanged
	var unchangedMsg models.MessageWaitsEth
	require.NoError(t, db.Where("signed_tx_hash = ?", "0xbbb").First(&unchangedMsg).Error)
	require.Equal(t, "confirmed", unchangedMsg.TxStatus)
	// TxSuccess should still be nil (wasn't touched by repair)
	require.Nil(t, unchangedMsg.TxSuccess)
}

func TestRepairProofSetCore_NilAddMessageIndex(t *testing.T) {
	ctx := context.Background()
	db := setupRepairTestDB(t)
	proofSetID := uint64(1)

	// Setup: Create message wait
	msgWait := models.MessageWaitsEth{SignedTxHash: "0xnil", TxStatus: "pending"}
	require.NoError(t, db.Create(&msgWait).Error)

	// Setup: Create proof set
	proofSet := models.PDPProofSet{ID: int64(proofSetID), CreateMessageHash: "0xnil", Service: "test"}
	require.NoError(t, db.Create(&proofSet).Error)

	// Setup: Root add entry with nil AddMessageIndex
	rootCID := testCID(t, "nil-index-root")
	rootAdd := models.PDPProofsetRootAdd{
		ProofsetID:      int64(proofSetID),
		AddMessageHash:  "0xnil",
		SubrootOffset:   0,
		Root:            rootCID.String(),
		AddMessageOK:    nil,
		AddMessageIndex: nil, // Explicitly nil
		Subroot:         testCID(t, "nil-index-subroot").String(),
		SubrootSize:     1024,
	}
	require.NoError(t, db.Create(&rootAdd).Error)

	mock := &mockActivePiecesProvider{
		count: big.NewInt(1),
		pieces: map[uint64]*smartcontracts.ActivePieces{
			0: {
				Pieces:   []cid.Cid{rootCID},
				PieceIds: []*big.Int{big.NewInt(77)},
				HasMore:  false,
			},
		},
	}

	// Execute - should not panic on nil AddMessageIndex
	result, err := repairProofSetCore(ctx, db, mock, proofSetID)

	// Assert
	require.NoError(t, err)
	require.Equal(t, 1, result.TotalRepaired)

	// Verify the root was created with AddMessageIndex = 0 (default)
	var roots []models.PDPProofsetRoot
	require.NoError(t, db.Where("proofset_id = ?", proofSetID).Find(&roots).Error)
	require.Len(t, roots, 1)
	require.Equal(t, int64(0), roots[0].AddMessageIndex)

	// Assert MessageWaitsEth: message is updated despite nil AddMessageIndex
	var updatedMsg models.MessageWaitsEth
	require.NoError(t, db.Where("signed_tx_hash = ?", "0xnil").First(&updatedMsg).Error)
	require.Equal(t, "confirmed", updatedMsg.TxStatus)
	require.NotNil(t, updatedMsg.TxSuccess)
	require.True(t, *updatedMsg.TxSuccess)
}

func TestRepairProofSetCore_MultipleMessageHashes(t *testing.T) {
	ctx := context.Background()
	db := setupRepairTestDB(t)
	proofSetID := uint64(1)

	// Setup: Create multiple message waits (all pending)
	msgWait1 := models.MessageWaitsEth{SignedTxHash: "0xmsg1", TxStatus: "pending"}
	require.NoError(t, db.Create(&msgWait1).Error)
	msgWait2 := models.MessageWaitsEth{SignedTxHash: "0xmsg2", TxStatus: "pending"}
	require.NoError(t, db.Create(&msgWait2).Error)
	msgWait3 := models.MessageWaitsEth{SignedTxHash: "0xmsg3", TxStatus: "pending"}
	require.NoError(t, db.Create(&msgWait3).Error)

	// Setup: Create proof set
	proofSet := models.PDPProofSet{ID: int64(proofSetID), CreateMessageHash: "0xmsg1", Service: "test"}
	require.NoError(t, db.Create(&proofSet).Error)

	// Setup: One root with multiple subroots, each using DIFFERENT message hashes
	rootCID := testCID(t, "multi-msg-root")
	pieceID := uint64(60)

	// Create 3 subroots with different message hashes
	messageHashes := []string{"0xmsg1", "0xmsg2", "0xmsg3"}
	for i := 0; i < 3; i++ {
		addMsgIndex := int64(0)
		rootAdd := models.PDPProofsetRootAdd{
			ProofsetID:      int64(proofSetID),
			AddMessageHash:  messageHashes[i],
			SubrootOffset:   int64(i * 1024),
			Root:            rootCID.String(),
			AddMessageOK:    nil,
			AddMessageIndex: &addMsgIndex,
			Subroot:         testCID(t, fmt.Sprintf("multi-msg-subroot-%d", i)).String(),
			SubrootSize:     1024,
		}
		require.NoError(t, db.Create(&rootAdd).Error)
	}

	mock := &mockActivePiecesProvider{
		count: big.NewInt(1),
		pieces: map[uint64]*smartcontracts.ActivePieces{
			0: {
				Pieces:   []cid.Cid{rootCID},
				PieceIds: []*big.Int{big.NewInt(int64(pieceID))},
				HasMore:  false,
			},
		},
	}

	// Execute
	result, err := repairProofSetCore(ctx, db, mock, proofSetID)

	// Assert repair succeeded
	require.NoError(t, err)
	require.Equal(t, 1, result.TotalRepaired)
	require.Equal(t, 3, result.RepairedEntries[0].Subroots)

	// Assert all 3 different message hashes were marked as confirmed
	for _, msgHash := range messageHashes {
		var updatedMsg models.MessageWaitsEth
		require.NoError(t, db.Where("signed_tx_hash = ?", msgHash).First(&updatedMsg).Error)
		require.Equal(t, "confirmed", updatedMsg.TxStatus, "message %s should be confirmed", msgHash)
		require.NotNil(t, updatedMsg.TxSuccess, "message %s should have TxSuccess set", msgHash)
		require.True(t, *updatedMsg.TxSuccess, "message %s should have TxSuccess=true", msgHash)
	}
}

func TestRepairProofSetCore_AlreadyConfirmedMessage(t *testing.T) {
	ctx := context.Background()
	db := setupRepairTestDB(t)
	proofSetID := uint64(1)

	// Setup: Create message wait that is ALREADY confirmed (not pending)
	// This simulates a message that was confirmed through normal flow but root wasn't created
	alreadyTrue := true
	msgWait := models.MessageWaitsEth{
		SignedTxHash: "0xalready",
		TxStatus:     "confirmed", // Already confirmed
		TxSuccess:    &alreadyTrue,
	}
	require.NoError(t, db.Create(&msgWait).Error)

	// Setup: Create proof set
	proofSet := models.PDPProofSet{ID: int64(proofSetID), CreateMessageHash: "0xalready", Service: "test"}
	require.NoError(t, db.Create(&proofSet).Error)

	// Setup: Root add entry referencing the already-confirmed message
	rootCID := testCID(t, "already-confirmed-root")
	addMsgIndex := int64(0)
	rootAdd := models.PDPProofsetRootAdd{
		ProofsetID:      int64(proofSetID),
		AddMessageHash:  "0xalready",
		SubrootOffset:   0,
		Root:            rootCID.String(),
		AddMessageOK:    nil,
		AddMessageIndex: &addMsgIndex,
		Subroot:         testCID(t, "already-confirmed-subroot").String(),
		SubrootSize:     1024,
	}
	require.NoError(t, db.Create(&rootAdd).Error)

	mock := &mockActivePiecesProvider{
		count: big.NewInt(1),
		pieces: map[uint64]*smartcontracts.ActivePieces{
			0: {
				Pieces:   []cid.Cid{rootCID},
				PieceIds: []*big.Int{big.NewInt(88)},
				HasMore:  false,
			},
		},
	}

	// Execute
	result, err := repairProofSetCore(ctx, db, mock, proofSetID)

	// Assert repair succeeded
	require.NoError(t, err)
	require.Equal(t, 1, result.TotalRepaired)

	// Assert the root was created
	var roots []models.PDPProofsetRoot
	require.NoError(t, db.Where("proofset_id = ?", proofSetID).Find(&roots).Error)
	require.Len(t, roots, 1)

	// Assert MessageWaitsEth: already-confirmed message was NOT modified
	// The WHERE clause `tx_status = "pending"` should prevent updates to confirmed messages
	var unchangedMsg models.MessageWaitsEth
	require.NoError(t, db.Where("signed_tx_hash = ?", "0xalready").First(&unchangedMsg).Error)
	require.Equal(t, "confirmed", unchangedMsg.TxStatus)
	// TxSuccess should still be true (the original value, not touched by repair)
	require.NotNil(t, unchangedMsg.TxSuccess)
	require.True(t, *unchangedMsg.TxSuccess)
}

func TestGetAllActivePiecesCore_EmptyResult(t *testing.T) {
	ctx := context.Background()

	mock := &mockActivePiecesProvider{
		count: big.NewInt(0),
		pieces: map[uint64]*smartcontracts.ActivePieces{
			0: {
				Pieces:   []cid.Cid{},
				PieceIds: []*big.Int{},
				HasMore:  false,
			},
		},
	}

	result, err := getAllActivePiecesCore(ctx, mock, 1)

	require.NoError(t, err)
	require.Empty(t, result)
}

func TestGetAllActivePiecesCore_PaginationError(t *testing.T) {
	ctx := context.Background()

	// Create valid first page data
	firstPageCIDs := make([]cid.Cid, 100)
	firstPageIDs := make([]*big.Int, 100)
	for i := 0; i < 100; i++ {
		firstPageCIDs[i] = testCID(t, fmt.Sprintf("page1-root-%d", i))
		firstPageIDs[i] = big.NewInt(int64(i))
	}

	mock := &mockActivePiecesProvider{
		count: big.NewInt(200),
		pieces: map[uint64]*smartcontracts.ActivePieces{
			0: {
				Pieces:   firstPageCIDs,
				PieceIds: firstPageIDs,
				HasMore:  true,
			},
			// Second page not in map - will return empty ActivePieces with HasMore=false
		},
	}

	// First page succeeds, second page returns empty (which gracefully stops pagination)
	result, err := getAllActivePiecesCore(ctx, mock, 1)

	require.NoError(t, err)
	// Should have returned what was fetched from first page
	require.Len(t, result, 100)
}

func TestGetAllActivePiecesCore_GetPiecesError(t *testing.T) {
	ctx := context.Background()

	mock := &mockActivePiecesProvider{
		count:     big.NewInt(100),
		piecesErr: fmt.Errorf("RPC connection failed"),
	}

	result, err := getAllActivePiecesCore(ctx, mock, 1)

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "RPC connection failed")
}
