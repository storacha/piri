package scheduler_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	logging "github.com/ipfs/go-log/v2"
	schedulerfx "github.com/storacha/piri/pkg/fx/scheduler"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/database"
	"github.com/storacha/piri/pkg/database/gormdb"
	"github.com/storacha/piri/pkg/pdp/service/models"
)

// MockTask implements TaskInterface for testing
type MockTask struct {
	typeDetails    scheduler.TaskTypeDetails
	mutex          sync.Mutex
	executedTasks  map[scheduler.TaskID]int // Maps task IDs to execution count
	shouldComplete bool                     // Whether tasks should complete or remain pending
	doFunc         func(taskID scheduler.TaskID) (bool, error)
	addTaskFunc    scheduler.AddTaskFunc
	readyForTasks  chan struct{} // Signal when addTaskFunc is set
}

func NewMockTask(name string, shouldComplete bool) *MockTask {
	return &MockTask{
		typeDetails: scheduler.TaskTypeDetails{
			Name: name,
			RetryWait: func(retries int) time.Duration {
				return time.Millisecond * 50
			},
		},
		executedTasks:  make(map[scheduler.TaskID]int),
		shouldComplete: shouldComplete,
		readyForTasks:  make(chan struct{}),
	}
}

func (m *MockTask) TypeDetails() scheduler.TaskTypeDetails {
	return m.typeDetails
}

func (m *MockTask) Adder(addTask scheduler.AddTaskFunc) {
	m.mutex.Lock()
	m.addTaskFunc = addTask
	m.mutex.Unlock()

	// Signal that addTaskFunc is ready
	close(m.readyForTasks)
}

func (m *MockTask) WaitForReady() {
	<-m.readyForTasks
}

func (m *MockTask) AddTask(extraInfo func(scheduler.TaskID, *gorm.DB) (bool, error)) {
	m.mutex.Lock()
	addFunc := m.addTaskFunc
	m.mutex.Unlock()

	if addFunc != nil {
		addFunc(extraInfo)
	}
}

func (m *MockTask) Do(taskID scheduler.TaskID) (done bool, err error) {
	if m.doFunc != nil {
		return m.doFunc(taskID)
	}
	//SleepRandom(time.Millisecond, 3*time.Second)

	m.mutex.Lock()
	m.executedTasks[taskID] = m.executedTasks[taskID] + 1
	m.mutex.Unlock()

	return m.shouldComplete, nil
}

func (m *MockTask) GetExecutionCount(taskID scheduler.TaskID) int {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.executedTasks[taskID]
}

func (m *MockTask) GetAllExecutedTasks() map[scheduler.TaskID]int {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Create a copy to avoid concurrent modification
	result := make(map[scheduler.TaskID]int, len(m.executedTasks))
	for k, v := range m.executedTasks {
		result[k] = v
	}
	return result
}

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	logging.SetAllLoggers(logging.LevelInfo)
	// Create a temporary file for the database
	tempDir, err := os.MkdirTemp("", "gorm-test-*")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(tempDir))
	})

	dbPath := filepath.Join(tempDir, "test.db")

	// Create a new GORM database with the specified options
	db, err := gormdb.New(dbPath, database.WithTimeout(time.Hour))
	require.NoError(t, err)

	// Create the Task table
	err = db.AutoMigrate(&models.Task{}, &models.TaskHistory{})
	require.NoError(t, err)

	return db
}

// TestTaskEngineBasicExecution tests that the engine correctly executes tasks
func TestTaskEngineBasicExecution(t *testing.T) {
	db := setupTestDB(t)

	// Create a mock task
	mockTask := NewMockTask("test_task", true)

	// Create the engine
	engine, err := scheduler.NewEngine(db, []scheduler.TaskInterface{mockTask})
	require.NoError(t, err)
	err = engine.Start(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := engine.Stop(context.Background()); err != nil {
			t.Logf("failed to stop engine: %v", err)
		}
	})

	// Wait for addTaskFunc to be set
	mockTask.WaitForReady()

	// Create some tasks in the database
	numTasks := 64
	for i := 0; i < numTasks; i++ {
		mockTask.AddTask(func(tID scheduler.TaskID, tx *gorm.DB) (bool, error) {
			// return true to indicate the task completed successfully without an error.
			return true, nil
		})
	}

	// Wait for all tasks to be executed and recorded in history
	assert.Eventually(t, func() bool {
		return len(mockTask.GetAllExecutedTasks()) == numTasks
	}, 50*time.Second, 500*time.Millisecond, "All tasks should be executed")

	// Wait for all executed tasks to be removed from the task table
	assert.Eventually(t, func() bool {
		var taskCount int64
		if err := db.Model(&models.Task{}).Count(&taskCount).Error; err != nil {
			return false
		}
		return taskCount == 0
	}, 5*time.Second, 100*time.Millisecond, fmt.Sprintf("All %d tasks should have been deleted from the database", numTasks))

	// Wait for each executed task to have an entry in task history
	assert.Eventually(t, func() bool {
		var historyCount int64
		if err := db.Model(&models.TaskHistory{}).Count(&historyCount).Error; err != nil {
			return false
		}
		return int(historyCount) == numTasks
	}, 5*time.Second, 100*time.Millisecond, fmt.Sprintf("All %d tasks should have an entry in TaskHistory", numTasks))
}

func TestTaskEngineResume(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(db *gorm.DB, currentSession string) []models.Task
		verify func(t *testing.T, db *gorm.DB, engine *scheduler.TaskEngine, mockTask *MockTask, initialTasks []models.Task)
	}{
		{
			name: "Tasks with different sessionID are resumed: simulates an ungraceful shutdown & resumption of engine",
			setup: func(db *gorm.DB, currentSession string) []models.Task {
				oldSession := "old-session-123"
				tasks := []models.Task{
					{
						Name:       "test_task",
						SessionID:  &oldSession,
						PostedTime: time.Now(),
						UpdateTime: time.Now(),
					},
					{
						Name:       "test_task",
						SessionID:  &oldSession,
						PostedTime: time.Now(),
						UpdateTime: time.Now(),
					},
				}
				for i := range tasks {
					require.NoError(t, db.Create(&tasks[i]).Error)
				}
				return tasks
			},
			verify: func(t *testing.T, db *gorm.DB, engine *scheduler.TaskEngine, mockTask *MockTask, initialTasks []models.Task) {
				// Wait for tasks to be executed
				assert.Eventually(t, func() bool {
					return len(mockTask.GetAllExecutedTasks()) == len(initialTasks)
				}, 5*time.Second, 500*time.Millisecond)

				// Verify tasks have been removed from database
				var remainingTasks []models.Task
				require.NoError(t, db.Find(&remainingTasks).Error)
				assert.Empty(t, remainingTasks)

				// assert that each executed task has an entry in task history
				historyCount := int64(0)
				require.NoError(t, db.Model(&models.TaskHistory{}).Count(&historyCount).Error)
				assert.EqualValuesf(t, len(initialTasks), historyCount, fmt.Sprintf("All %d tasks should have an entry in TaskHistory", len(initialTasks)))
			},
		},
		{
			name: "Tasks without sessionID are executed: simulates graceful shutdown & resumption of engine",
			setup: func(db *gorm.DB, currentSession string) []models.Task {
				tasks := []models.Task{
					{
						Name:       "test_task",
						SessionID:  nil,
						PostedTime: time.Now(),
						UpdateTime: time.Now(),
					},
					{
						Name:       "test_task",
						SessionID:  nil,
						PostedTime: time.Now(),
						UpdateTime: time.Now(),
					},
				}
				for i := range tasks {
					require.NoError(t, db.Create(&tasks[i]).Error)
				}
				return tasks
			},
			verify: func(t *testing.T, db *gorm.DB, engine *scheduler.TaskEngine, mockTask *MockTask, initialTasks []models.Task) {
				// Wait for tasks to be executed
				assert.Eventually(t, func() bool {
					return len(mockTask.GetAllExecutedTasks()) == len(initialTasks)
				}, 5*time.Second, 500*time.Millisecond)

				// Verify tasks have been removed from database
				var remainingTasks []models.Task
				require.NoError(t, db.Find(&remainingTasks).Error)
				assert.Empty(t, remainingTasks)

				// assert that each executed task has an entry in task history
				historyCount := int64(0)
				require.NoError(t, db.Model(&models.TaskHistory{}).Count(&historyCount).Error)
				assert.EqualValuesf(t, len(initialTasks), historyCount, fmt.Sprintf("All %d tasks should have an entry in TaskHistory", len(initialTasks)))
			},
		},
		{
			name: "Mixed tasks are handled correctly",
			setup: func(db *gorm.DB, currentSession string) []models.Task {
				oldSession := "old-session-456"
				tasks := []models.Task{
					// expected execution given old sessionID
					{
						Name:       "test_task",
						SessionID:  &oldSession,
						PostedTime: time.Now(),
						UpdateTime: time.Now(),
					},
					// expect execution given no sessionID
					{
						Name:       "test_task",
						SessionID:  nil,
						PostedTime: time.Now(),
						UpdateTime: time.Now(),
					},
					// no execution given same sessionID
					{
						Name:       "test_task",
						SessionID:  &currentSession,
						PostedTime: time.Now(),
						UpdateTime: time.Now(),
					},
				}
				for i := range tasks {
					require.NoError(t, db.Create(&tasks[i]).Error)
				}
				return tasks
			},
			verify: func(t *testing.T, db *gorm.DB, engine *scheduler.TaskEngine, mockTask *MockTask, initialTasks []models.Task) {
				// Should execute 2 tasks (old session + nil session)
				// The current session task should not be executed
				assert.Eventually(t, func() bool {
					return len(mockTask.GetAllExecutedTasks()) == 2
				}, 5*time.Second, 500*time.Millisecond)

				// Verify only the current session task remains
				var remainingTasks []models.Task
				require.NoError(t, db.Find(&remainingTasks).Error)
				assert.Len(t, remainingTasks, 1)
				assert.Equal(t, engine.SessionID(), *remainingTasks[0].SessionID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)

			// Create mock task that completes successfully
			mockTask := NewMockTask("test_task", true)

			// Create the engine
			engine, err := scheduler.NewEngine(db, []scheduler.TaskInterface{mockTask})
			require.NoError(t, err)

			// Create tasks using the engine's actual session ID
			initialTasks := tt.setup(db, engine.SessionID())
			err = engine.Start(t.Context())
			require.NoError(t, err)
			t.Cleanup(func() {
				if err := engine.Stop(context.Background()); err != nil {
					t.Logf("failed to stop engine: %v", err)
				}
			})

			// Wait for the engine to be ready
			mockTask.WaitForReady()

			// Run verification
			tt.verify(t, db, engine, mockTask, initialTasks)
		})
	}
}

// TestTaskEngineRetryFailedTasks verifies that failed tasks are properly retried
// and that SessionID is correctly set to nil when a task fails
func TestTaskEngineRetryFailedTasks(t *testing.T) {
	tests := []struct {
		name             string
		maxFailures      uint
		failureThreshold int // How many times the task should fail before succeeding (-1 = always fail)
		expectedAttempts int // Total expected execution attempts
		verifyHistory    func(t *testing.T, histories []models.TaskHistory)
	}{
		{
			name:             "task fails once then succeeds",
			maxFailures:      3,
			failureThreshold: 1,
			expectedAttempts: 2,
			verifyHistory: func(t *testing.T, histories []models.TaskHistory) {
				require.Len(t, histories, 2)
				// First attempt should fail
				assert.False(t, histories[0].Result)
				assert.Contains(t, histories[0].Err, "simulated failure on attempt 1")
				// Second attempt should succeed
				assert.True(t, histories[1].Result)
				assert.Empty(t, histories[1].Err)
			},
		},
		{
			name:             "task fails 4 times then succeeds on 5th attempt",
			maxFailures:      5,
			failureThreshold: 4,
			expectedAttempts: 5,
			verifyHistory: func(t *testing.T, histories []models.TaskHistory) {
				require.Len(t, histories, 5)
				// First 4 attempts should fail
				for i := 0; i < 4; i++ {
					assert.False(t, histories[i].Result)
					assert.Contains(t, histories[i].Err, fmt.Sprintf("simulated failure on attempt %d", i+1))
				}
				// 5th attempt should succeed
				assert.True(t, histories[4].Result)
				assert.Empty(t, histories[4].Err)
			},
		},
		{
			name:             "task never succeeds and exceeds max retries",
			maxFailures:      3,
			failureThreshold: -1, // Always fail
			expectedAttempts: 4,  // Initial attempt + 3 retries
			verifyHistory: func(t *testing.T, histories []models.TaskHistory) {
				require.Len(t, histories, 4)
				// All attempts should fail
				for i := 0; i < len(histories); i++ {
					assert.False(t, histories[i].Result)
					assert.Contains(t, histories[i].Err, fmt.Sprintf("simulated failure on attempt %d", i+1))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)

			// Track execution attempts for each task
			executionAttempts := make(map[scheduler.TaskID]int)
			var mu sync.Mutex

			// Create a mock task with custom failure behavior
			mockTask := NewMockTask("test_task", false)
			mockTask.typeDetails.MaxFailures = tt.maxFailures
			mockTask.doFunc = func(taskID scheduler.TaskID) (bool, error) {
				mu.Lock()
				attempts := executionAttempts[taskID]
				executionAttempts[taskID] = attempts + 1
				mu.Unlock()

				if tt.failureThreshold == -1 || attempts < tt.failureThreshold {
					// Task should fail
					return false, fmt.Errorf("simulated failure on attempt %d", attempts+1)
				}
				// Task succeeds
				return true, nil
			}

			// Create the engine
			engine, err := scheduler.NewEngine(db, []scheduler.TaskInterface{mockTask})
			require.NoError(t, err)
			err = engine.Start(t.Context())
			require.NoError(t, err)
			t.Cleanup(func() {
				if err := engine.Stop(context.Background()); err != nil {
					t.Logf("failed to stop engine: %v", err)
				}
			})

			// Wait for addTaskFunc to be set
			mockTask.WaitForReady()

			// Create a task
			mockTask.AddTask(func(tID scheduler.TaskID, tx *gorm.DB) (bool, error) {
				return true, nil
			})

			// Wait for expected number of execution attempts
			assert.Eventually(t, func() bool {
				mu.Lock()
				defer mu.Unlock()
				for _, attempts := range executionAttempts {
					if attempts >= tt.expectedAttempts {
						return true
					}
				}
				return false
			}, 20*time.Second, 100*time.Millisecond, fmt.Sprintf("Task should be attempted %d times", tt.expectedAttempts))

			// Small delay to ensure final database updates are complete
			time.Sleep(200 * time.Millisecond)

			// Verify task deletion status
			var count int64
			db.Model(&models.Task{}).Where("name = ?", "test_task").Count(&count)
			assert.Equal(t, int64(0), count, "Task should be deleted")

			// Verify task history
			var histories []models.TaskHistory
			require.NoError(t, db.Where("name = ?", "test_task").Order("work_start ASC").Find(&histories).Error)

			// Call the test case's verification function
			tt.verifyHistory(t, histories)
		})
	}
}

// Setup and start the task engine. engine go vroom.
func setupAndStartEngine(t *testing.T, task scheduler.TaskInterface) *fxtest.App {
	var engine *scheduler.TaskEngine
	app := fxtest.New(t,
		fx.NopLogger,
		fx.Provide(
			schedulerfx.ProvideEngine,
			fx.Annotate(
				func() *gorm.DB {
					return setupTestDB(t)
				},
				fx.ResultTags(`name:"engine_db"`),
			),
			fx.Annotate(
				func() scheduler.TaskInterface {
					return task
				},
				fx.ResultTags(`group:"scheduler_tasks"`),
			),
		),
		// we populate the engine, to force a dependency on it, causing it to initialize
		fx.Populate(&engine),
	)

	app.RequireStart()

	return app

}

// TestTaskEngineGracefulShutdown verifies that the engine waits for active tasks
// to complete before shutting down gracefully
func TestTaskEngineGracefulShutdown(t *testing.T) {
	// Channel to control task execution
	taskComplete := make(chan struct{})
	taskStarted := make(chan struct{})

	// Create a mock task that we can control
	mockTask := NewMockTask("test_task", false)
	mockTask.doFunc = func(taskID scheduler.TaskID) (bool, error) {
		// Signal that task has started
		close(taskStarted)
		// Wait for signal to complete
		<-taskComplete
		return true, nil
	}

	app := setupAndStartEngine(t, mockTask)

	mockTask.WaitForReady()

	// Add a task that will start executing
	mockTask.AddTask(func(tID scheduler.TaskID, tx *gorm.DB) (bool, error) {
		return true, nil
	})

	// Wait for task to start executing
	require.Eventually(t, func() bool {
		<-taskStarted
		return true
	}, time.Second, 100*time.Millisecond, "Task should be started")

	shutdownComplete := make(chan error, 1)
	go func() {
		shutdownComplete <- app.Stop(t.Context())
	}()

	// Give the engine a moment to start shutdown process
	time.Sleep(100 * time.Millisecond)

	// Verify that engine is waiting for the task (activeTasks > 0)
	// We can't directly access activeTasks, but we can observe behavior
	// The shutdown should not complete yet
	select {
	case err := <-shutdownComplete:
		t.Fatalf("Shutdown completed too early, should wait for task: %v", err)
	case <-time.After(500 * time.Millisecond):
		// yay, shutdown is waiting
	}

	// Now complete the task
	close(taskComplete)

	select {
	case err := <-shutdownComplete:
		require.NoError(t, err, "Shutdown should complete successfully after task finishes")
	case <-time.After(3 * time.Second):
		t.Fatal("Shutdown did not complete after task finished")
	}
}

// TestTaskEngineShutdownTimeout verifies that the engine respects the shutdown timeout
// when tasks don't complete in time
func TestTaskEngineShutdownTimeout(t *testing.T) {
	// Channel to control task execution (never closed, so task never completes)
	taskStarted := make(chan struct{})
	taskNeverCompletes := make(chan struct{})

	// Create a mock task that never completes
	mockTask := NewMockTask("test_task", false)
	mockTask.doFunc = func(taskID scheduler.TaskID) (bool, error) {
		// Signal that task has started
		close(taskStarted)
		// Wait forever (this channel is never closed)
		<-taskNeverCompletes
		return true, nil
	}

	stopTimeout := time.Second
	app := setupAndStartEngine(t, mockTask)

	mockTask.WaitForReady()

	// Add a task that will start executing and never complete
	mockTask.AddTask(func(tID scheduler.TaskID, tx *gorm.DB) (bool, error) {
		return true, nil
	})

	// Wait for task to start executing
	require.Eventually(t, func() bool {
		<-taskStarted
		return true
	}, time.Second, 100*time.Millisecond, "Task should be started")

	ctx, cancel := context.WithTimeout(t.Context(), stopTimeout)
	defer cancel()

	err := app.Stop(ctx)

	// Verify that we got a timeout error
	assert.Error(t, err, "Shutdown should fail with timeout")
	assert.Contains(t, err.Error(), "context deadline exceeded", "Should get context deadline exceeded error from fx.StopTimeout")
}

// TestTaskEngineGracefulShutdownMultipleTasks verifies that the engine waits for ALL active tasks
// to complete before shutting down gracefully, not just a single task
func TestTaskEngineGracefulShutdownMultipleTasks(t *testing.T) {
	const numTasks = 5

	// Create channels to control each task independently
	taskCompletions := make([]chan struct{}, numTasks)
	taskStarted := make([]chan struct{}, numTasks)
	for i := range taskCompletions {
		taskCompletions[i] = make(chan struct{})
		taskStarted[i] = make(chan struct{})
	}

	// Track which task is being executed
	taskExecutions := make(map[scheduler.TaskID]int)
	var taskExecutionsMu sync.Mutex

	// Create a mock task that tracks multiple concurrent executions
	mockTask := NewMockTask("test_task", false) // MaxConcurrency of 10 to allow all tasks to run
	mockTask.doFunc = func(taskID scheduler.TaskID) (bool, error) {
		// Determine which task index this is
		taskExecutionsMu.Lock()
		taskIndex := len(taskExecutions)
		taskExecutions[taskID] = taskIndex
		taskExecutionsMu.Unlock()

		if taskIndex >= numTasks {
			// Extra task, complete immediately
			return true, nil
		}

		// Signal that this specific task has started
		close(taskStarted[taskIndex])

		// Wait for signal to complete this specific task
		<-taskCompletions[taskIndex]
		return true, nil
	}

	app := setupAndStartEngine(t, mockTask)

	mockTask.WaitForReady()

	// Add multiple tasks that will start executing
	for i := 0; i < numTasks; i++ {
		mockTask.AddTask(func(tID scheduler.TaskID, tx *gorm.DB) (bool, error) {
			return true, nil
		})
	}

	// Wait for all tasks to start executing
	for i := 0; i < numTasks; i++ {
		select {
		case <-taskStarted[i]:
			// Task i is now running
		case <-time.After(5 * time.Second):
			t.Fatalf("Task %d did not start within timeout", i)
		}
	}

	shutdownComplete := make(chan error, 1)
	go func() {
		shutdownComplete <- app.Stop(context.Background())
	}()

	// Give the engine a moment to start shutdown process
	time.Sleep(100 * time.Millisecond)

	// Verify that shutdown is waiting (should not complete yet)
	select {
	case err := <-shutdownComplete:
		t.Fatalf("Shutdown completed too early with %d tasks still running: %v", numTasks, err)
	case <-time.After(500 * time.Millisecond):
		//  shutdown is waiting for all tasks
	}

	// Complete tasks one by one, except the last one
	for i := 0; i < numTasks-1; i++ {
		close(taskCompletions[i])

		// After completing each task (except the last), shutdown should still be waiting
		time.Sleep(100 * time.Millisecond)
		select {
		case err := <-shutdownComplete:
			t.Fatalf("Shutdown completed too early after completing %d/%d tasks: %v", i+1, numTasks, err)
		case <-time.After(200 * time.Millisecond):
			// Good, still waiting for remaining tasks
		}
	}

	// Now complete the last task
	close(taskCompletions[numTasks-1])

	// Shutdown should now complete since all tasks are done
	select {
	case err := <-shutdownComplete:
		require.NoError(t, err, "Shutdown should complete successfully after all tasks finish")
	case <-time.After(3 * time.Second):
		t.Fatal("Shutdown did not complete after all tasks finished")
	}

	// Verify all tasks were executed
	taskExecutionsMu.Lock()
	assert.Equal(t, numTasks, len(taskExecutions), "All tasks should have been executed")
	taskExecutionsMu.Unlock()
}

// TestTaskEngineShutdownTimeoutMultipleTasks verifies that the engine respects the shutdown timeout
// even when multiple tasks don't complete in time
func TestTaskEngineShutdownTimeoutMultipleTasks(t *testing.T) {
	const numTasks = 3

	// Create channels for task control (never closed, so tasks never complete)
	taskStarted := make([]chan struct{}, numTasks)
	taskNeverCompletes := make([]chan struct{}, numTasks)
	for i := range taskStarted {
		taskStarted[i] = make(chan struct{})
		taskNeverCompletes[i] = make(chan struct{})
	}

	taskCount := 0
	var taskCountMu sync.Mutex

	// Create a mock task that never completes
	mockTask := NewMockTask("test_task", false)
	mockTask.doFunc = func(taskID scheduler.TaskID) (bool, error) {
		taskCountMu.Lock()
		index := taskCount
		taskCount++
		taskCountMu.Unlock()

		if index >= numTasks {
			// Extra task, complete immediately
			return true, nil
		}

		// Signal that this task has started
		close(taskStarted[index])
		// Wait forever (this channel is never closed)
		<-taskNeverCompletes[index]
		return true, nil
	}

	app := setupAndStartEngine(t, mockTask)

	mockTask.WaitForReady()

	// Add multiple tasks that will never complete
	for i := 0; i < numTasks; i++ {
		mockTask.AddTask(func(tID scheduler.TaskID, tx *gorm.DB) (bool, error) {
			return true, nil
		})
	}

	// Wait for all tasks to start executing
	for i := 0; i < numTasks; i++ {
		select {
		case <-taskStarted[i]:
			// Task is now running and will never complete
		case <-time.After(5 * time.Second):
			t.Fatalf("Task %d did not start within timeout", i)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err := app.Stop(ctx)

	// Verify that we got a timeout error from
	assert.Error(t, err, "Shutdown should fail with timeout")
	assert.Contains(t, err.Error(), "context deadline exceeded", "Should get context deadline exceeded error")

	// Verify the expected number of tasks were running
	taskCountMu.Lock()
	assert.Equal(t, numTasks, taskCount, "All tasks should have started")
	taskCountMu.Unlock()
}
