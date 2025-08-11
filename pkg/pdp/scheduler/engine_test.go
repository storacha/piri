package scheduler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/database"
	"github.com/storacha/piri/pkg/database/gormdb"
	"github.com/storacha/piri/pkg/pdp/service/models"
)

// MockTask implements TaskInterface for testing
type MockTask struct {
	typeDetails    TaskTypeDetails
	mutex          sync.Mutex
	executedTasks  map[TaskID]int // Maps task IDs to execution count
	shouldComplete bool           // Whether tasks should complete or remain pending
	doFunc         func(taskID TaskID) (bool, error)
	addTaskFunc    AddTaskFunc
	readyForTasks  chan struct{} // Signal when addTaskFunc is set
}

func NewMockTask(name string, maxConcurrent int, shouldComplete bool) *MockTask {
	return &MockTask{
		typeDetails: TaskTypeDetails{
			Name: name,
			RetryWait: func(retries int) time.Duration {
				return time.Millisecond * 50
			},
		},
		executedTasks:  make(map[TaskID]int),
		shouldComplete: shouldComplete,
		readyForTasks:  make(chan struct{}),
	}
}

func (m *MockTask) TypeDetails() TaskTypeDetails {
	return m.typeDetails
}

func (m *MockTask) Adder(addTask AddTaskFunc) {
	m.mutex.Lock()
	m.addTaskFunc = addTask
	m.mutex.Unlock()

	// Signal that addTaskFunc is ready
	close(m.readyForTasks)
}

func (m *MockTask) WaitForReady() {
	<-m.readyForTasks
}

func (m *MockTask) AddTask(extraInfo func(TaskID, *gorm.DB) (bool, error)) {
	m.mutex.Lock()
	addFunc := m.addTaskFunc
	m.mutex.Unlock()

	if addFunc != nil {
		addFunc(extraInfo)
	}
}

func (m *MockTask) Do(taskID TaskID) (done bool, err error) {
	if m.doFunc != nil {
		return m.doFunc(taskID)
	}
	//SleepRandom(time.Millisecond, 3*time.Second)

	m.mutex.Lock()
	m.executedTasks[taskID] = m.executedTasks[taskID] + 1
	m.mutex.Unlock()

	return m.shouldComplete, nil
}

func (m *MockTask) GetExecutionCount(taskID TaskID) int {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.executedTasks[taskID]
}

func (m *MockTask) GetAllExecutedTasks() map[TaskID]int {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Create a copy to avoid concurrent modification
	result := make(map[TaskID]int, len(m.executedTasks))
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
	err = db.AutoMigrate(&models.Task{}, &models.TaskImpl{}, &models.TaskFollow{}, &models.TaskHistory{})
	require.NoError(t, err)

	return db
}

// TestTaskEngineBasicExecution tests that the engine correctly executes tasks
func TestTaskEngineBasicExecution(t *testing.T) {
	db := setupTestDB(t)

	// Create a mock task
	mockTask := NewMockTask("test_task", 5, true)

	// Create the engine
	engine, err := NewEngine(db, []TaskInterface{mockTask})
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
		mockTask.AddTask(func(tID TaskID, tx *gorm.DB) (bool, error) {
			// return true to indicate the task completed successfully without an error.
			return true, nil
		})
	}

	// within 5 seconds we expect all tasks to execute, or we fail, query every 500ms
	assert.Eventually(t, func() bool {
		var count int64
		if err := db.Model(&models.TaskHistory{}).Count(&count).Error; err != nil {
			return false
		}
		return int(count) == len(mockTask.GetAllExecutedTasks())
	}, 50*time.Second, 500*time.Millisecond)

	// assert that all executed tasks have been removed from the task table
	taskCount := int64(0)
	require.NoError(t, db.Model(&models.Task{}).Count(&taskCount).Error)
	assert.Zero(t, taskCount, fmt.Sprintf("All %d tasks should have been deleted from the database, instead found: %d", numTasks, taskCount))

	// assert that each executed task has an entry in task history
	historyCount := int64(0)
	require.NoError(t, db.Model(&models.TaskHistory{}).Count(&historyCount).Error)
	assert.EqualValuesf(t, numTasks, historyCount, fmt.Sprintf("All %d tasks should have an entry in TaskHistory", numTasks))
}

func TestTaskEngineResume(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(db *gorm.DB, currentSession string) []models.Task
		verify func(t *testing.T, db *gorm.DB, engine *TaskEngine, mockTask *MockTask, initialTasks []models.Task)
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
			verify: func(t *testing.T, db *gorm.DB, engine *TaskEngine, mockTask *MockTask, initialTasks []models.Task) {
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
			verify: func(t *testing.T, db *gorm.DB, engine *TaskEngine, mockTask *MockTask, initialTasks []models.Task) {
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
			verify: func(t *testing.T, db *gorm.DB, engine *TaskEngine, mockTask *MockTask, initialTasks []models.Task) {
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
			mockTask := NewMockTask("test_task", 5, true)

			// Create the engine
			engine, err := NewEngine(db, []TaskInterface{mockTask})
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
			executionAttempts := make(map[TaskID]int)
			var mu sync.Mutex

			// Create a mock task with custom failure behavior
			mockTask := NewMockTask("test_task", 5, false)
			mockTask.typeDetails.MaxFailures = tt.maxFailures
			mockTask.doFunc = func(taskID TaskID) (bool, error) {
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
			engine, err := NewEngine(db, []TaskInterface{mockTask})
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
			mockTask.AddTask(func(tID TaskID, tx *gorm.DB) (bool, error) {
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
