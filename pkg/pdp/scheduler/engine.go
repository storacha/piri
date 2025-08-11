// Package scheduler implements a session-based task scheduler with the following features:
//
// 1. Session-Based Ownership: Each engine instance gets a globally unique session ID
// 2. Clean Session Boundaries: Tasks are tied to specific sessions, not just owners
// 3. Automatic Cleanup: Previous sessions are cleaned up on startup
// 4. Graceful Termination: Tasks are released when an engine shuts down
package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	logging "github.com/ipfs/go-log/v2"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/models"
)

var log = logging.Logger("pdp/scheduler")

// TaskEngine is the central scheduler.
type TaskEngine struct {
	ctx       context.Context
	cancel    context.CancelFunc
	db        *gorm.DB
	sessionID string
	handlers  []*taskTypeHandler
}

// Option is a functional option for configuring a TaskEngine.
type Option func(*TaskEngine) error

// WithSessionID sets a custom session ID for the TaskEngine.
// If not provided, a new UUID v7 will be generated.
func WithSessionID(sessionID string) Option {
	return func(e *TaskEngine) error {
		e.sessionID = sessionID
		return nil
	}
}

// NewEngine creates a new TaskEngine with the provided task implementations.
// The engine manages task scheduling with session-based ownership, ensuring
// clean boundaries between different engine instances and automatic cleanup
// of tasks from previous sessions.
//
// Parameters:
//   - db: The database connection for task persistence
//   - impls: Task implementations that define the work to be scheduled
//   - opts: Optional configuration (e.g., WithSessionID)
func NewEngine(db *gorm.DB, impls []TaskInterface, opts ...Option) (*TaskEngine, error) {
	e := &TaskEngine{
		sessionID: mustGenerateSessionID(),
		db:        db,
	}

	for _, opt := range opts {
		if err := opt(e); err != nil {
			return nil, err
		}
	}

	for _, impl := range impls {
		h := &taskTypeHandler{
			TaskInterface:   impl,
			TaskTypeDetails: impl.TypeDetails(),
			TaskEngine:      e,
		}
		e.handlers = append(e.handlers, h)
	}

	return e, nil
}

// Start initializes and begins the task engine's operation.
//
// The provided context is used only for database migration and session cleanup
// during startup. Callers should pass a context with a timeout if they want to
// limit how long startup operations can take.
//
// After startup completes, the engine creates its own internal context for
// ongoing operations, which is separate from the startup context. To stop the
// engine, callers must use the Stop method rather than canceling the startup context.
//
// Start performs the following operations:
//   1. Runs database migrations
//   2. Cleans up tasks from previous sessions
//   3. Starts task adders for each registered task type
//   4. Starts periodic schedulers if configured
//   5. Begins the main polling loop for unassigned tasks
func (e *TaskEngine) Start(ctx context.Context) error {
	log.Infof("Starting engine with session ID: %s", e.sessionID)
	if err := models.AutoMigrateDB(ctx, e.db); err != nil {
		return fmt.Errorf("auto migrate db: %w", err)
	}

	if err := e.cleanupPreviousSessions(ctx); err != nil {
		return fmt.Errorf("failed to cleanup previous sessions: %w", err)
	}

	e.ctx, e.cancel = context.WithCancel(context.Background())
	for _, h := range e.handlers {
		h.Adder(h.AddTask)

		if h.TaskTypeDetails.PeriodicScheduler != nil {
			go h.runPeriodicTask()
		}
	}

	go e.poller()
	return nil
}

// SessionID returns the unique session ID of this engine instance.
// This ID is used to track task ownership and ensure clean session boundaries.
func (e *TaskEngine) SessionID() string {
	return e.sessionID
}

// Stop gracefully shuts down the task engine.
// It stops accepting new work and releases all tasks owned by this session,
// making them available for other engine instances to pick up.
// The context parameter can be used to set a timeout for the shutdown operation.
func (e *TaskEngine) Stop(ctx context.Context) error {
	log.Debugw("Stopping task engine", "session_id", e.sessionID)
	// Stop accepting new work
	e.cancel()

	// Release all tasks owned by this session
	if err := e.db.
		WithContext(ctx).
		Model(&models.Task{}).
		Where("session_id = ?", e.sessionID).
		Updates(map[string]interface{}{
			"session_id": nil,
		}).Error; err != nil {
		return fmt.Errorf("failed to release tasks during shutdown: %w", err)
	}
	log.Infow("Stopped task engine", "session_id", e.sessionID)
	return nil
}

// poller is the main work loop that continuously checks for unassigned tasks.
// It uses an adaptive polling strategy: polling more frequently when work is found
// (100ms) and less frequently when idle (3s).
func (e *TaskEngine) poller() {
	pollDuration := 3 * time.Second
	pollNextDuration := 100 * time.Millisecond
	nextWait := pollNextDuration

	for {
		select {
		case <-time.After(nextWait):
		case <-e.ctx.Done():
			return
		}
		nextWait = pollDuration

		accepted := e.pollerTryAllWork()
		if accepted {
			nextWait = pollNextDuration
		}
	}
}

// pollerTryAllWork attempts to find and schedule unassigned tasks for all registered task types.
// It returns true if any work was accepted, which signals the poller to check again sooner.
// Tasks are fetched in order of update time to ensure fair scheduling.
func (e *TaskEngine) pollerTryAllWork() bool {
	for _, h := range e.handlers {
		var tasks []models.Task
		if err := e.db.WithContext(e.ctx).
			Where("name = ? AND session_id IS NULL", h.TaskTypeDetails.Name).
			Order("update_time").
			Find(&tasks).Error; err != nil {
			log.Errorf("Unable to read work for task type %s: %v", h.TaskTypeDetails.Name, err)
			continue
		}

		var taskIDs []TaskID
		for _, t := range tasks {
			if h.TaskTypeDetails.RetryWait == nil ||
				time.Since(t.UpdateTime) > h.TaskTypeDetails.RetryWait(0) {
				taskIDs = append(taskIDs, TaskID(t.ID))
			}
		}

		if len(taskIDs) > 0 {
			accepted := h.considerWork(taskIDs, e.db)
			if accepted {
				return true
			}
			log.Warnf("Work not accepted for %d %s task(s)", len(taskIDs), h.TaskTypeDetails.Name)
		}
	}

	return false
}

// cleanupPreviousSessions releases tasks that were owned by previous engine sessions.
// This ensures that if an engine instance crashes or stops ungracefully, its tasks
// can be picked up by new engine instances. Only tasks with session IDs different
// from the current session are released.
func (e *TaskEngine) cleanupPreviousSessions(ctx context.Context) error {
	result := e.db.
		WithContext(ctx).
		Model(&models.Task{}).
		Where("session_id IS NOT NULL AND session_id != ?", e.sessionID).
		Updates(map[string]interface{}{
			"session_id": nil,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to release tasks from previous sessions: %w", result.Error)
	}

	if result.RowsAffected > 0 {
		log.Infof("Released %d tasks from previous sessions", result.RowsAffected)
	}

	return nil
}

// mustGenerateSessionID generates a new UUID v7 for use as a session identifier.
// It panics if UUID generation fails, as a session ID is critical for engine operation.
func mustGenerateSessionID() string {
	id, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	idstr := id.String()
	if len(idstr) == 0 {
		panic("invalid session id")
	}
	return idstr
}
