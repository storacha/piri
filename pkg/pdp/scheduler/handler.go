package scheduler

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/database"
	"github.com/storacha/piri/pkg/pdp/service/models"
)

// taskTypeHandler ties a task implementation with engine-specific metadata.
type taskTypeHandler struct {
	TaskInterface
	TaskTypeDetails TaskTypeDetails
	TaskEngine      *TaskEngine
	doneMu          sync.Mutex
}

// AddTask is the implementation passed to each task's Adder.
// It creates a new task record in the database.
func (h *taskTypeHandler) AddTask(extra func(TaskID, *gorm.DB) (bool, error)) {
	var tID TaskID
	retryWait := 100 * time.Millisecond

retryAddTask:
	err := h.TaskEngine.db.WithContext(h.TaskEngine.ctx).Transaction(func(tx *gorm.DB) error {
		// Create a new Task record.
		task := models.Task{
			PostedTime: time.Now(),
			AddedBy:    h.TaskEngine.sessionID,
			Name:       h.TaskTypeDetails.Name,
		}

		// Insert the task and let GORM fill in the auto-generated ID.
		if err := tx.Create(&task).Error; err != nil {
			return fmt.Errorf("could not insert task: %w", err)
		}

		// Set the task ID from the newly inserted record.
		tID = TaskID(task.ID)

		// Call the extra callback to update additional info in the same transaction.
		shouldCommit, err := extra(tID, tx)
		if err != nil {
			return err
		}

		if shouldCommit {
			return nil
		}

		/*
		   - AddTask attempts to add a task to the database.
		   - AddTask invokes a tasks Adder function (extra in this method).
		     The Adder function, implemented by each tasks, is used to determine if said task should run.
		   - If the task needs to run, its Adder returns true, otherwise false.
		   - In the event the Adder returns false we don't want to run the task, so we return an error here to abort
		      the database transactions which creates the task in the db.
		*/
		return ErrDoNotCommit
	})
	if err != nil {
		// If a unique constraint error is detected, assume the task already exists.
		if database.IsUniqueConstraintError(err) {
			log.Debugf("addtask(%s) saw unique constraint, so it's added already.", h.TaskTypeDetails.Name)
			return
		}
		// If it's a timeout error, backoff and retry.
		if database.IsLockedError(err) {
			log.Warnf("addtask(%s) saw locked error retrying in %s.", retryWait, h.TaskTypeDetails.Name)
			time.Sleep(retryWait)
			retryWait *= 2
			goto retryAddTask
		}

		if errors.Is(err, ErrDoNotCommit) {
			return
		}
		log.Errorw("Could not add task. AddTask func failed", "error", err, "type", h.TaskTypeDetails.Name)
		return
	}

}

// considerWork claims and executes tasks.
func (h *taskTypeHandler) considerWork(taskIDs []TaskID, db *gorm.DB) bool {
	acceptedAny := false

	log.Debugf("Considering work for tasks %v", taskIDs)
	for _, id := range taskIDs {
		log.Debugf("Considering work for task %d", id)
		result := db.
			WithContext(h.TaskEngine.ctx).
			Model(&models.Task{}).
			Where(&models.Task{ID: int64(id), SessionID: nil}).
			Updates(models.Task{
				SessionID:  &h.TaskEngine.sessionID,
				UpdateTime: time.Now(),
			})

		if result.Error != nil {
			log.Errorw("Could not claim task", "task_id", id, "error", result.Error)
			continue
		}
		if result.RowsAffected == 0 {
			// Already taken by someone else (or in race condition). Skip it.
			log.Debugf("Task %d was already claimed; skipping", id)
			continue
		}

		// Successfully claimed this task, so let’s run it in a goroutine:
		acceptedAny = true
		go func(taskID TaskID) {
			h.TaskEngine.activeTasks.Add(1)
			defer h.TaskEngine.activeTasks.Add(-1)
			tlog := log.With("name", h.TaskTypeDetails.Name, "task_id", taskID, "session_id", h.TaskEngine.sessionID)
			var (
				done    bool
				doErr   error
				doStart = time.Now()
			)
			defer func() {
				if r := recover(); r != nil {
					stackSlice := make([]byte, 4092)
					sz := runtime.Stack(stackSlice, false)
					tlog.Error("Task recovered from panic", "panic", r, "stack", string(stackSlice[:sz]))
				}

				h.doneMu.Lock()
				defer h.doneMu.Unlock()
				if err := h.handleDoneTask(taskID, doStart, done, doErr); err != nil {
					log.Errorw("failed to handle task done", "task_id", taskID, "error", err)
				}
			}()

			tlog.Info("Task starting execution")
			done, doErr = h.Do(taskID)
			if doErr != nil {
				tlog.Errorw("Task execution failed", "error", doErr, "done", done, "duration", time.Since(doStart))
			}
		}(id)
	}

	return acceptedAny
}

func (h *taskTypeHandler) handleDoneTask(id TaskID, startTime time.Time, done bool, doErr error) error {
	tlog := log.With(
		"name", h.TaskTypeDetails.Name,
		"task_id", id,
		"session_id", h.TaskEngine.sessionID,
		"done", done,
		"duration", time.Since(startTime),
	)

	var (
		endTime         = time.Now()
		retryWait       = 100 * time.Millisecond
		maxRetries uint = 10
		retryCount uint = 0
	)

retryHandleDoneTask:
	err := h.TaskEngine.db.WithContext(h.TaskEngine.ctx).Transaction(func(tx *gorm.DB) error {
		// find the task that we are handling
		task := models.Task{}
		if res := tx.Model(&models.Task{}).
			Where("id = ?", id).
			First(&task); res.Error != nil {
			return fmt.Errorf("failed to handle task: failed to query taskID: %d: %w", id, res.Error)
		} else if res.RowsAffected == 0 {
			return fmt.Errorf("failed to handle task: no task found for taskID: %d: %w", id, res.Error)
		}

		taskErrMsg := ""
		if done {
			// if the task is done, we can delete it
			if err := tx.Delete(&models.Task{ID: int64(id)}).Error; err != nil {
				return fmt.Errorf("failed to handle done task: failed to delete task %d: %w", id, err)
			}
			// the task may have returned an error, in addition to completing successfully, record this if present
			if doErr != nil {
				taskErrMsg = "non-failing error: " + doErr.Error()
				tlog.Warn("Task completed execution with error", "error", doErr)
			} else {
				tlog.Info("Task completed execution")
			}
		} else {
			// if the task is not done, see if it can be retried, and capture its error message
			if doErr != nil {
				taskErrMsg = "error: " + doErr.Error()
			}
			// the task has exceeded the number of allowed retries, delete it
			if h.TaskTypeDetails.MaxFailures > 0 && task.Retries >= h.TaskTypeDetails.MaxFailures {
				tlog.Errorw("Task execution retries exceeded, removing task", "maxFailures", h.TaskTypeDetails.MaxFailures, "retries", task.Retries, "error", doErr)
				if err := tx.Delete(&models.Task{ID: int64(id)}).Error; err != nil {
					return fmt.Errorf("failed to deleted failed task %d: %w", id, err)
				}
			} else {
				tlog.Warnw("Task retrying execution", "maxFailures", h.TaskTypeDetails.MaxFailures, "retry", task.Retries, "error", doErr)
				// the task may be retried, increment retry counter and set sessionID to nil, allowing the task engine
				// to pick it back up and try again
				if err := tx.Model(&models.Task{}).
					Where(&models.Task{ID: int64(id)}).
					Select("session_id", "retries", "update_time").
					Updates(models.Task{
						SessionID:  nil,
						Retries:    task.Retries + 1,
						UpdateTime: time.Now(),
					}).Error; err != nil {
					return fmt.Errorf("failed to updated failed task %d: %w", id, err)
				}
			}
		}

		// record history about the task.
		if res := tx.Create(&models.TaskHistory{
			TaskID:               int64(id),
			Name:                 task.Name,
			Posted:               task.PostedTime,
			WorkStart:            startTime,
			WorkEnd:              endTime,
			Result:               done,
			Err:                  taskErrMsg,
			CompletedBySessionID: h.TaskEngine.sessionID,
		}); res.Error != nil {
			return fmt.Errorf("failed to write task (%d) history: %w", id, res.Error)
		}

		return nil
	})
	if err != nil {
		// If it's a serialization error and we haven't exceeded max retries, backoff and retry
		if database.IsLockedError(err) && retryCount < maxRetries {
			retryCount++
			tlog.Warnw("handleDoneTask transaction failed with serialization error, retrying",
				"error", err, "retry_count", retryCount, "max_retries", maxRetries)
			time.Sleep(retryWait)
			retryWait *= 2
			goto retryHandleDoneTask
		}
		return err
	}

	return nil
}

var ErrDoNotCommit = errors.New("do not commit")

// runPeriodicTask runs a periodic task at the specified interval
func (h *taskTypeHandler) runPeriodicTask() {
	scheduler := h.TaskTypeDetails.PeriodicScheduler
	if scheduler == nil {
		return
	}

	ticker := time.NewTicker(scheduler.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.TaskEngine.ctx.Done():
			return
		case <-ticker.C:
			h.TaskEngine.activeTasks.Add(1)
			err := scheduler.Runner(h.AddTask)
			h.TaskEngine.activeTasks.Add(-1)
			if err != nil {
				log.Warnf("Periodic scheduler for task %s returned error: %v",
					h.TaskTypeDetails.Name, err)
			}
		}
	}
}
