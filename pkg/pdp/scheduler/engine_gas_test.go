package scheduler_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service/models"
)

// TestTaskEngineGasTooHighDoesNotIncrementRetries tests AC2: when a task returns
// ErrGasTooHigh, the handler requeues the task WITHOUT incrementing the retry counter.
//
// Scenario: MaxFailures=2, task returns ErrGasTooHigh 3 times then succeeds.
// If gas deferrals counted as failures, the task would be killed after 2 failures.
// Since they don't count, the task should survive and eventually succeed.
func TestTaskEngineGasTooHighDoesNotIncrementRetries(t *testing.T) {
	db := setupTestDB(t)

	gasFailures := 3 // Number of times to return ErrGasTooHigh before succeeding

	var mu sync.Mutex
	attempts := make(map[scheduler.TaskID]int)

	mockTask := NewMockTask("test_task", false)
	mockTask.typeDetails.MaxFailures = 2 // Less than gasFailures — would kill if counted
	mockTask.doFunc = func(taskID scheduler.TaskID) (bool, error) {
		mu.Lock()
		attempt := attempts[taskID]
		attempts[taskID] = attempt + 1
		mu.Unlock()

		if attempt < gasFailures {
			return false, scheduler.ErrGasTooHigh
		}
		// Succeed on the 4th attempt
		return true, nil
	}

	engine, err := scheduler.NewEngine(db, []scheduler.TaskInterface{mockTask})
	require.NoError(t, err)
	err = engine.Start(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := engine.Stop(context.Background()); err != nil {
			t.Logf("failed to stop engine: %v", err)
		}
	})

	mockTask.WaitForReady()

	// Add one task
	mockTask.AddTask(func(tID scheduler.TaskID, tx *gorm.DB) (bool, error) {
		return true, nil
	})

	// Wait for the task to eventually succeed (should take gasFailures+1 attempts)
	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, count := range attempts {
			if count >= gasFailures+1 {
				return true
			}
		}
		return false
	}, 30*time.Second, 100*time.Millisecond,
		fmt.Sprintf("Task should be attempted at least %d times", gasFailures+1))

	// Small delay to ensure final database updates are complete
	time.Sleep(500 * time.Millisecond)

	// Task should be deleted (completed successfully)
	var taskCount int64
	db.Model(&models.Task{}).Where("name = ?", "test_task").Count(&taskCount)
	assert.Equal(t, int64(0), taskCount, "Task should be deleted after successful completion")

	// The task should have a successful completion in history
	var histories []models.TaskHistory
	require.NoError(t, db.Where("name = ?", "test_task").Order("work_start ASC").Find(&histories).Error)

	// We expect gasFailures+1 history entries total
	require.Len(t, histories, gasFailures+1,
		"should have %d history entries (3 gas deferrals + 1 success)", gasFailures+1)

	// The last entry should be successful
	lastHistory := histories[len(histories)-1]
	assert.True(t, lastHistory.Result, "last attempt should succeed")
	assert.Empty(t, lastHistory.Err, "last attempt should have no error")
}

// TestTaskEngineGasTooHighRetryCounterNotIncremented tests AC2 more directly:
// verify that the retry counter in the task table is NOT incremented when
// ErrGasTooHigh is returned.
func TestTaskEngineGasTooHighRetryCounterNotIncremented(t *testing.T) {
	db := setupTestDB(t)

	var mu sync.Mutex
	attempts := make(map[scheduler.TaskID]int)
	// Use a channel to control when we check the DB state
	gasReturned := make(chan struct{}, 10)

	mockTask := NewMockTask("test_task", false)
	mockTask.typeDetails.MaxFailures = 5
	mockTask.doFunc = func(taskID scheduler.TaskID) (bool, error) {
		mu.Lock()
		attempt := attempts[taskID]
		attempts[taskID] = attempt + 1
		mu.Unlock()

		if attempt < 3 {
			gasReturned <- struct{}{}
			return false, scheduler.ErrGasTooHigh
		}
		return true, nil
	}

	engine, err := scheduler.NewEngine(db, []scheduler.TaskInterface{mockTask})
	require.NoError(t, err)
	err = engine.Start(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := engine.Stop(context.Background()); err != nil {
			t.Logf("failed to stop engine: %v", err)
		}
	})

	mockTask.WaitForReady()

	mockTask.AddTask(func(tID scheduler.TaskID, tx *gorm.DB) (bool, error) {
		return true, nil
	})

	// Wait for at least one gas deferral to happen
	select {
	case <-gasReturned:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for gas deferral")
	}

	// Give the handler time to process
	time.Sleep(200 * time.Millisecond)

	// Check that retry counter was NOT incremented
	var task models.Task
	result := db.Where("name = ?", "test_task").First(&task)
	if result.Error == nil {
		// Task still exists (hasn't completed yet)
		assert.Equal(t, uint(0), task.Retries,
			"retry counter should NOT be incremented for ErrGasTooHigh")
	}
	// If the task already completed, that's also fine - it wasn't killed by MaxFailures
}

// TestTaskEngineGasTooHighVsRealFailure tests that ErrGasTooHigh and real failures
// are handled differently. Real failures increment retries; gas deferrals do not.
//
// MaxFailures=1 means the task is killed after 1 real-failure retry. The handler
// checks `retries >= MaxFailures` BEFORE incrementing, so the sequence is:
//
//	attempt 0: gas  -> retries stays 0
//	attempt 1: gas  -> retries stays 0
//	attempt 2: real -> retries 0->1 (check 0>=1 false, requeue)
//	attempt 3: real -> check 1>=1 true, killed
//
// Total: 4 attempts, all non-successful.
func TestTaskEngineGasTooHighVsRealFailure(t *testing.T) {
	db := setupTestDB(t)

	var mu sync.Mutex
	attempts := make(map[scheduler.TaskID]int)

	mockTask := NewMockTask("test_task", false)
	mockTask.typeDetails.MaxFailures = 1
	mockTask.doFunc = func(taskID scheduler.TaskID) (bool, error) {
		mu.Lock()
		attempt := attempts[taskID]
		attempts[taskID] = attempt + 1
		mu.Unlock()

		switch attempt {
		case 0:
			// First: gas too high (should NOT count as failure)
			return false, scheduler.ErrGasTooHigh
		case 1:
			// Second: gas too high again (should NOT count as failure)
			return false, scheduler.ErrGasTooHigh
		case 2:
			// Third: real failure (SHOULD count as failure, retry #1)
			return false, fmt.Errorf("real error")
		case 3:
			// Fourth: real failure — but killed before requeue since retries(1) >= MaxFailures(1)
			return false, fmt.Errorf("real error again")
		default:
			// Should not reach here — MaxFailures=1 kills the task after attempt 3
			return true, nil
		}
	}

	engine, err := scheduler.NewEngine(db, []scheduler.TaskInterface{mockTask})
	require.NoError(t, err)
	err = engine.Start(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := engine.Stop(context.Background()); err != nil {
			t.Logf("failed to stop engine: %v", err)
		}
	})

	mockTask.WaitForReady()

	mockTask.AddTask(func(tID scheduler.TaskID, tx *gorm.DB) (bool, error) {
		return true, nil
	})

	// Wait for enough attempts
	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, count := range attempts {
			// We expect at least 4 attempts:
			// 2 gas deferrals (no retry increment) + 2 real failures (retries incremented)
			// After retry #1 with MaxFailures=1, task should be killed
			if count >= 4 {
				return true
			}
		}
		return false
	}, 30*time.Second, 100*time.Millisecond)

	// Wait for handler to finish processing
	time.Sleep(500 * time.Millisecond)

	// Task should be deleted (killed after MaxFailures real failures)
	var taskCount int64
	db.Model(&models.Task{}).Where("name = ?", "test_task").Count(&taskCount)
	assert.Equal(t, int64(0), taskCount,
		"Task should be deleted after exceeding real failure max")

	// Verify history: should have 4 entries (2 gas + 2 real failures)
	var histories []models.TaskHistory
	require.NoError(t, db.Where("name = ?", "test_task").Order("work_start ASC").Find(&histories).Error)
	require.Len(t, histories, 4, "should have 4 history entries total")

	// None of the history entries should show as "done" (all were failures/deferrals)
	for _, h := range histories {
		assert.False(t, h.Result, "no attempt should have succeeded")
	}
}
