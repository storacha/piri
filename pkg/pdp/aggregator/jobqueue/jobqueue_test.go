package jobqueue_test

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue"
	internaltesting "github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/internal/testing"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/queue"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/serializer"
	"github.com/stretchr/testify/require"
)

// TestMessage is a simple test message type
type TestMessage struct {
	ID      string
	Payload string
	Delay   time.Duration
}

// newTestJobQueue creates a new JobQueue for testing
func newTestJobQueue(t *testing.T, db *sql.DB, opts ...jobqueue.Option) *jobqueue.JobQueue[TestMessage] {
	t.Helper()
	if db == nil {
		db = internaltesting.NewInMemoryDB(t)
		// Setup queue schema
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()
		require.NoError(t, queue.Setup(ctx, db))
	}

	ser := serializer.JSON[TestMessage]{}
	jq, err := jobqueue.New[TestMessage]("test-queue", db, ser, opts...)
	require.NoError(t, err)
	return jq
}

func TestJobQueue_Stop_GracefulShutdown(t *testing.T) {
	t.Run("waits for running tasks to complete", func(t *testing.T) {
		jq := newTestJobQueue(t, nil, jobqueue.WithMaxWorkers(1))

		var taskCompleted atomic.Bool
		var taskStarted atomic.Bool

		// Register a task that takes some time
		err := jq.Register("task", func(ctx context.Context, msg TestMessage) error {
			taskStarted.Store(true)
			// Simulate work that takes time
			time.Sleep(500 * time.Millisecond)
			taskCompleted.Store(true)
			return nil
		})
		require.NoError(t, err)

		// Start the queue
		ctx := context.Background()
		require.NoError(t, jq.Start(ctx))

		// Enqueue a task
		err = jq.Enqueue(ctx, "task", TestMessage{ID: "1", Payload: "test"})
		require.NoError(t, err)

		// Wait for task to start
		require.Eventually(t, func() bool {
			return taskStarted.Load()
		}, 15*time.Second, 250*time.Millisecond, "timed out waiting for task to start")

		// Stop the queue (should wait for task to complete)
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err = jq.Stop(stopCtx)
		require.NoError(t, err)

		// Verify task was allowed to complete
		require.True(t, taskCompleted.Load(), "Task should have completed before Stop returned")
	})
}

func TestJobQueue_Stop_RejectsNewTasks(t *testing.T) {
	t.Run("rejects enqueue after Stop is called", func(t *testing.T) {
		jq := newTestJobQueue(t, nil)

		// Register a simple task
		err := jq.Register("simple-task", func(ctx context.Context, msg TestMessage) error {
			return nil
		})
		require.NoError(t, err)

		// Start and immediately stop
		ctx := context.Background()
		require.NoError(t, jq.Start(ctx))

		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err = jq.Stop(stopCtx)
		require.NoError(t, err)

		// Try to enqueue a task after stopping
		err = jq.Enqueue(ctx, "simple-task", TestMessage{ID: "1", Payload: "test"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "job queue is stopping")
	})
}

func TestJobQueue_RejectRegisterAfterStart(t *testing.T) {
	t.Run("rejects register before Start is called", func(t *testing.T) {
		jq := newTestJobQueue(t, nil)

		// Register a simple task, should pass
		err := jq.Register("simple-task", func(ctx context.Context, msg TestMessage) error {
			return nil
		})
		require.NoError(t, err)

		// Start the queue
		ctx := context.Background()
		require.NoError(t, jq.Start(ctx))

		// Register a simple task, should fail as job queue is running
		err = jq.Register("simple-task", func(ctx context.Context, msg TestMessage) error {
			return nil
		})
		require.Error(t, err)

	})
}

func TestJobQueue_Stop_ContextTimeout(t *testing.T) {
	t.Run("returns timeout error when context expires", func(t *testing.T) {
		jq := newTestJobQueue(t, nil, jobqueue.WithMaxWorkers(1))

		blockForever := make(chan struct{})

		// Register a task that blocks forever
		err := jq.Register("block-forever", func(ctx context.Context, msg TestMessage) error {
			<-blockForever
			return nil
		})
		require.NoError(t, err)

		// Start the queue
		ctx := context.Background()
		require.NoError(t, jq.Start(ctx))

		// Enqueue a blocking task
		err = jq.Enqueue(ctx, "block-forever", TestMessage{ID: "1", Payload: "test"})
		require.NoError(t, err)

		// Wait a moment for task to start processing
		time.Sleep(time.Second)

		// Try to stop with a short timeout
		stopCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		err = jq.Stop(stopCtx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "stop timeout")

		// Clean up
		close(blockForever)
	})
}

func TestJobQueue_Stop_MultipleCallsHandled(t *testing.T) {
	t.Run("handles multiple Stop calls gracefully", func(t *testing.T) {
		jq := newTestJobQueue(t, nil)

		// Register a simple task
		err := jq.Register("simple-task", func(ctx context.Context, msg TestMessage) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		})
		require.NoError(t, err)

		// Start the queue
		ctx := context.Background()
		require.NoError(t, jq.Start(ctx))

		// Call Stop multiple times concurrently
		var wg sync.WaitGroup
		errs := make([]error, 3)

		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				errs[idx] = jq.Stop(stopCtx)
			}(i)
		}

		wg.Wait()

		// First call should succeed, others should get "already stopping" error
		successCount := 0
		alreadyStoppingCount := 0

		for _, err := range errs {
			if err == nil {
				successCount++
			} else if err.Error() == "job queue is already stopping" {
				alreadyStoppingCount++
			}
		}

		require.Equal(t, 1, successCount, "exactly one Stop call should succeed")
		require.Equal(t, 2, alreadyStoppingCount, "other Stop calls should get 'already stopping' error")
	})
}

func TestJobQueue_Stop_CompletesAllPendingTasks(t *testing.T) {
	t.Run("processes all tasks before shutdown", func(t *testing.T) {
		jq := newTestJobQueue(t, nil, jobqueue.WithMaxWorkers(2))

		var processedCount atomic.Int32
		taskProcessing := make(chan struct{}, 10)

		// Register a task that tracks processing
		err := jq.Register("count-task", func(ctx context.Context, msg TestMessage) error {
			taskProcessing <- struct{}{}
			time.Sleep(50 * time.Millisecond) // Simulate work
			processedCount.Add(1)
			return nil
		})
		require.NoError(t, err)

		// Start the queue
		ctx := context.Background()
		require.NoError(t, jq.Start(ctx))

		// Enqueue multiple tasks
		expectedTasks := 5
		for i := 0; i < expectedTasks; i++ {
			err = jq.Enqueue(ctx, "count-task", TestMessage{
				ID:      string(rune('0' + i)),
				Payload: "test",
			})
			require.NoError(t, err)
		}

		// Wait for all tasks to be picked up and start processing
		for i := 0; i < expectedTasks; i++ {
			select {
			case <-taskProcessing:
				// Task started
			case <-time.After(2 * time.Second):
				t.Fatalf("Only %d tasks started out of %d", i, expectedTasks)
			}
		}

		// Stop the queue
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = jq.Stop(stopCtx)
		require.NoError(t, err)

		// Verify all enqueued tasks were processed
		require.Equal(t, int32(expectedTasks), processedCount.Load())
	})
}

func TestJobQueue_Stop_EnqueueDuringShutdown(t *testing.T) {
	t.Run("rejects new tasks during shutdown", func(t *testing.T) {
		jq := newTestJobQueue(t, nil, jobqueue.WithMaxWorkers(1))

		shutdownStarted := make(chan struct{})
		taskCanComplete := make(chan struct{})

		// Register a task that signals when running
		err := jq.Register("slow-task", func(ctx context.Context, msg TestMessage) error {
			close(shutdownStarted)
			<-taskCanComplete
			return nil
		})
		require.NoError(t, err)

		// Start the queue
		ctx := context.Background()
		require.NoError(t, jq.Start(ctx))

		// Enqueue a task
		err = jq.Enqueue(ctx, "slow-task", TestMessage{ID: "1", Payload: "test"})
		require.NoError(t, err)

		// Wait for task to start
		<-shutdownStarted

		// Start stopping in a goroutine
		stopDone := make(chan error, 1)
		go func() {
			stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			stopDone <- jq.Stop(stopCtx)
		}()

		// Give Stop a moment to mark as stopping
		time.Sleep(200 * time.Millisecond)

		// Try to enqueue during shutdown
		err = jq.Enqueue(ctx, "slow-task", TestMessage{ID: "2", Payload: "test"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "job queue is stopping")

		// Complete the running task
		close(taskCanComplete)

		// Wait for Stop to complete
		select {
		case err := <-stopDone:
			require.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("Stop did not complete")
		}
	})
}

func TestJobQueue_Stop_WithoutStart(t *testing.T) {
	t.Run("can stop without starting", func(t *testing.T) {
		jq := newTestJobQueue(t, nil)

		// Stop without starting must fail
		stopCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		err := jq.Stop(stopCtx)
		require.Error(t, err)

		// enqueueing a job into an unstarted queue must fail
		ctx := context.Background()
		err = jq.Enqueue(ctx, "test", TestMessage{ID: "1", Payload: "test"})
		require.Error(t, err)
	})
}

func TestJobQueue_Stop_TaskFailureHandling(t *testing.T) {
	t.Run("completes shutdown even if tasks fail", func(t *testing.T) {
		jq := newTestJobQueue(t, nil,
			jobqueue.WithMaxWorkers(2))

		var processedCount atomic.Int32

		// Register tasks that fail
		err := jq.Register("failing-task", func(ctx context.Context, msg TestMessage) error {
			processedCount.Add(1)
			return errors.New("task failed")
		})
		require.NoError(t, err)

		// Start the queue
		ctx := context.Background()
		require.NoError(t, jq.Start(ctx))

		// Enqueue multiple tasks that will fail
		for i := 0; i < 3; i++ {
			err = jq.Enqueue(ctx, "failing-task", TestMessage{
				ID:      string(rune('0' + i)),
				Payload: "test",
			})
			require.NoError(t, err)
		}

		// Give tasks time to be processed
		time.Sleep(500 * time.Millisecond)

		// Stop should complete successfully even though tasks failed
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err = jq.Stop(stopCtx)
		require.NoError(t, err)

		// Verify tasks were attempted
		require.GreaterOrEqual(t, processedCount.Load(), int32(3))
	})
}

func TestJobQueue_StartStopStartCycle(t *testing.T) {
	t.Run("cannot start after stop", func(t *testing.T) {
		jq := newTestJobQueue(t, nil)

		var processedCount atomic.Int32

		// Register a simple task
		err := jq.Register("simple-task", func(ctx context.Context, msg TestMessage) error {
			processedCount.Add(1)
			return nil
		})
		require.NoError(t, err)

		// First cycle: Start, enqueue, stop
		ctx := context.Background()
		require.NoError(t, jq.Start(ctx))

		err = jq.Enqueue(ctx, "simple-task", TestMessage{ID: "1", Payload: "first"})
		require.NoError(t, err)

		time.Sleep(300 * time.Millisecond) // Let task process

		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = jq.Stop(stopCtx)
		cancel()
		require.NoError(t, err)

		require.Equal(t, int32(1), processedCount.Load())

		// Note: In the current implementation, we cannot reStart after Stop
		// because stopping is permanent. This test documents this behavior.
		// If restart capability is needed, the JobQueue struct would need
		// to reset the stopping flag in Start() method.

		// Starting after stopping must fail
		require.Error(t, jq.Start(ctx))

		// Enqueue will fail because queue is stopped.
		err = jq.Enqueue(ctx, "simple-task", TestMessage{ID: "2", Payload: "second"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "job queue is stopping")
	})
}

func TestJobQueue_WithOnFailure(t *testing.T) {
	t.Run("calls OnFailure callback after exhausting all retries", func(t *testing.T) {
		// Create job queue with low max retries for faster test
		jq := newTestJobQueue(t, nil,
			jobqueue.WithMaxRetries(2),
			jobqueue.WithMaxTimeout(100*time.Millisecond))

		var (
			failureCount    atomic.Int32
			onFailureCalled atomic.Bool
			capturedErr     error
			capturedMsg     TestMessage
			mu              sync.Mutex
		)

		// Register a task that always fails, with OnFailure callback
		err := jq.Register("always-failing-task",
			func(ctx context.Context, msg TestMessage) error {
				failureCount.Add(1)
				return errors.New("intentional failure")
			},
			jobqueue.WithOnFailure(func(ctx context.Context, msg TestMessage, err error) error {
				onFailureCalled.Store(true)
				mu.Lock()
				capturedErr = err
				capturedMsg = msg
				mu.Unlock()
				return nil
			}),
		)
		require.NoError(t, err)

		// Start the queue
		ctx := context.Background()
		require.NoError(t, jq.Start(ctx))

		// Enqueue a task that will fail
		testMsg := TestMessage{ID: "fail-test", Payload: "should trigger OnFailure"}
		err = jq.Enqueue(ctx, "always-failing-task", testMsg)
		require.NoError(t, err)

		// Wait for task to be processed and retried
		// Give enough time for retries with backoff
		require.Eventually(t, func() bool {
			return onFailureCalled.Load()
		}, 15*time.Second, 250*time.Millisecond, "OnFailure callback should have been triggered")

		// Verify the callback received the correct error and message
		mu.Lock()
		require.Error(t, capturedErr)
		require.Contains(t, capturedErr.Error(), "intentional failure")
		require.Equal(t, testMsg.ID, capturedMsg.ID)
		require.Equal(t, testMsg.Payload, capturedMsg.Payload)
		mu.Unlock()

		// Verify the task was attempted the correct number of times
		// With MaxRetries=2, it appears to be total attempts, not additional retries
		require.GreaterOrEqual(t, failureCount.Load(), int32(2), "Task should have been attempted at least 2 times")

		// Clean up
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		require.NoError(t, jq.Stop(stopCtx))
	})
}
