package models_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/storacha/piri/pkg/database/gormdb"
	"github.com/storacha/piri/pkg/pdp/service/models"
)

func TestOnDeleteSetNull(t *testing.T) {
	// Create a temporary file for the database
	tempDir, err := os.MkdirTemp("", "gorm-test-*")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(tempDir))
	})

	dbPath := filepath.Join(tempDir, "test.db")

	// Create a new GORM database
	db, err := gormdb.New(dbPath)
	require.NoError(t, err)

	err = models.AutoMigrateDB(t.Context(), db)
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	// Create a task
	task := &models.Task{
		Name:       "test-task",
		AddedBy:    "test",
		PostedTime: nowFunc(),
	}
	result := db.Create(task)
	require.NoError(t, result.Error)
	// first taskID in sequence on new DB is always 1
	require.EqualValues(t, 1, task.ID)

	// Create all the models that have SET NULL relationships with Task

	// ParkedPiece.TaskID
	piece := &models.ParkedPiece{
		PieceCID:        "test-cid-1",
		PiecePaddedSize: 1024,
		PieceRawSize:    1024,
		TaskID:          &task.ID,
	}
	result = db.Create(piece)
	require.NoError(t, result.Error)
	require.NotZero(t, piece.ID)

	// ParkedPiece.CleanupTaskID
	pieceWithCleanup := &models.ParkedPiece{
		PieceCID:        "test-cid-2",
		PiecePaddedSize: 2048,
		PieceRawSize:    2048,
		CleanupTaskID:   &task.ID,
		LongTerm:        true,
	}
	result = db.Create(pieceWithCleanup)
	require.NoError(t, result.Error)
	require.NotZero(t, pieceWithCleanup.ID)

	// PDPProofSet.ChallengeRequestTaskID
	proofSet := &models.PDPProofSet{
		Service:                "test-service",
		CreateMessageHash:      "test-hash",
		ChallengeRequestTaskID: &task.ID,
	}
	result = db.Create(proofSet)
	require.NoError(t, result.Error)
	require.NotZero(t, proofSet.ID)

	// PDPProveTask with relationship to task
	proveTask := &models.PDPProveTask{
		ProofsetID: proofSet.ID,
		TaskID:     task.ID,
	}
	result = db.Create(proveTask)
	require.NoError(t, result.Error)

	// Verify all relationships exist, each model with a taskID fk references the task.ID
	var retrievedPiece models.ParkedPiece
	result = db.First(&retrievedPiece, piece.ID)
	require.NoError(t, result.Error)
	assert.Equal(t, task.ID, *retrievedPiece.TaskID)

	var retrievedPieceWithCleanup models.ParkedPiece
	result = db.First(&retrievedPieceWithCleanup, pieceWithCleanup.ID)
	require.NoError(t, result.Error)
	assert.Equal(t, task.ID, *retrievedPieceWithCleanup.CleanupTaskID)

	var retrievedProofSet models.PDPProofSet
	result = db.First(&retrievedProofSet, proofSet.ID)
	require.NoError(t, result.Error)
	assert.Equal(t, task.ID, *retrievedProofSet.ChallengeRequestTaskID)

	// Delete the task
	result = db.Delete(task)
	require.NoError(t, result.Error)

	// Verify all foreign keys are set to NULL
	result = db.First(&retrievedPiece, piece.ID)
	require.NoError(t, result.Error)
	assert.Nil(t, retrievedPiece.TaskID)

	result = db.First(&retrievedPieceWithCleanup, pieceWithCleanup.ID)
	require.NoError(t, result.Error)
	assert.Nil(t, retrievedPieceWithCleanup.CleanupTaskID)

	result = db.First(&retrievedProofSet, proofSet.ID)
	require.NoError(t, result.Error)
	assert.Nil(t, retrievedProofSet.ChallengeRequestTaskID)

	// For PDPProveTask, we expect it to be deleted (CASCADE)
	var numProveTasks int64
	db.Model(&models.PDPProveTask{}).Count(&numProveTasks)
	assert.NoError(t, result.Error)
	assert.Zero(t, numProveTasks)

}

func TestOnDeleteCascade(t *testing.T) {
	// Create a temporary file for the database
	tempDir, err := os.MkdirTemp("", "gorm-test-*")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(tempDir))
	})

	dbPath := filepath.Join(tempDir, "test.db")

	// Create a new GORM database
	db, err := gormdb.New(dbPath)
	require.NoError(t, err)

	err = models.AutoMigrateDB(t.Context(), db)
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	// Create a ParkedPiece
	piece := &models.ParkedPiece{
		PieceCID:        "test-cid",
		PiecePaddedSize: 1024,
		PieceRawSize:    1024,
	}
	result := db.Create(piece)
	require.NoError(t, result.Error)
	require.NotZero(t, piece.ID)

	// Create a ParkedPieceRef with a reference to the ParkedPiece
	pieceRef := &models.ParkedPieceRef{
		PieceID:     piece.ID,
		DataURL:     "test-url",
		DataHeaders: []byte("{}"),
	}
	result = db.Create(pieceRef)
	require.NoError(t, result.Error)
	require.NotZero(t, pieceRef.RefID)

	// Verify the relationship exists
	var retrievedPieceRef models.ParkedPieceRef
	result = db.First(&retrievedPieceRef, pieceRef.RefID)
	require.NoError(t, result.Error)
	assert.Equal(t, piece.ID, retrievedPieceRef.PieceID)

	// Delete the piece
	result = db.Delete(piece)
	require.NoError(t, result.Error)

	// Verify the reference was deleted (CASCADE)
	result = db.First(&retrievedPieceRef, pieceRef.RefID)
	assert.Error(t, result.Error)
	assert.True(t, result.RowsAffected == 0)
}

// Helper function to mock the current time for consistent testing
func nowFunc() time.Time {
	return time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
}
