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

type Option func(*TaskEngine) error

func WithSessionID(sessionID string) Option {
	return func(e *TaskEngine) error {
		e.sessionID = sessionID
		return nil
	}
}

// NewEngine creates a new TaskEngine with the provided task implementations.
func NewEngine(db *gorm.DB, impls []TaskInterface, opts ...Option) (*TaskEngine, error) {
	ctx, cancel := context.WithCancel(context.Background())
	e := &TaskEngine{
		ctx:       ctx,
		sessionID: mustGenerateSessionID(),
		cancel:    cancel,
		db:        db,
	}

	for _, opt := range opts {
		if err := opt(e); err != nil {
			cancel()
			return nil, err
		}
	}

	log.Infof("Starting engine with session ID: %s", e.sessionID)

	// TODO add dedicated start method to the engine that actually runs these routines
	// Clean up tasks from previous sessions
	if err := e.cleanupPreviousSessions(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to cleanup previous sessions: %w", err)
	}

	for _, impl := range impls {
		h := &taskTypeHandler{
			TaskInterface:   impl,
			TaskTypeDetails: impl.TypeDetails(),
			TaskEngine:      e,
		}
		e.handlers = append(e.handlers, h)

		// Start the adder routine for the task type.
		go h.Adder(h.AddTask)

		// Start the periodic scheduler if provided
		if h.TaskTypeDetails.PeriodicScheduler != nil {
			go h.runPeriodicTask()
		}
	}

	go e.poller()

	return e, nil
}

// SessionID returns the unique session ID of this engine instance
func (e *TaskEngine) SessionID() string {
	return e.sessionID
}

// GracefullyTerminate stops new task scheduling and releases owned tasks.
func (e *TaskEngine) GracefullyTerminate() {
	// Stop accepting new work
	e.cancel()

	// Release all tasks owned by this session
	if err := e.db.Model(&models.Task{}).
		Where("session_id = ?", e.sessionID).
		Updates(map[string]interface{}{
			"session_id": nil,
		}).Error; err != nil {
		log.Errorf("Failed to release tasks during shutdown: %v", err)
	} else {
		log.Infof("Released tasks for session %s", e.sessionID)
	}
}

// poller continuously checks for work.
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
		// Here you could also call a follow-up work routine if needed.
	}
}

// pollerTryAllWork looks for unassigned tasks in the DB and schedules them.
func (e *TaskEngine) pollerTryAllWork() bool {
	// Iterate over all registered task types.
	for _, h := range e.handlers {
		var tasks []models.Task
		// Fetch tasks for this type that are unassigned (no session).
		if err := e.db.WithContext(e.ctx).
			Where("name = ? AND session_id IS NULL", h.TaskTypeDetails.Name).
			Order("update_time").
			Find(&tasks).Error; err != nil {
			log.Errorf("Unable to read work for task type %s: %v", h.TaskTypeDetails.Name, err)
			continue
		}

		var taskIDs []TaskID
		// Filter tasks based on retry logic.
		// Since the Task model no longer has a retries field, we assume a default value (e.g., 0)
		// or adjust the logic if you retrieve retries from another source.
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

// cleanupPreviousSessions releases tasks from previous sessions
func (e *TaskEngine) cleanupPreviousSessions() error {
	// Release all tasks from previous sessions (any session ID != current)
	result := e.db.Model(&models.Task{}).
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
