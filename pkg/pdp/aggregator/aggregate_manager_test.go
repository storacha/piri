package aggregator_test

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/raulk/clock"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/piri/pkg/pdp/aggregator"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

// mockQueue is a simple implementation of jobqueue.Service for testing
type mockQueue struct {
	taskHandler   aggregator.TaskHandler
	mu            sync.Mutex
	delay         time.Duration // Simulated processing delay
	failRate      float32       // Failure rate (0-1) for error injection
	enqueuedCount atomic.Int64
}

func (mq *mockQueue) Start(ctx context.Context) error { return nil }
func (mq *mockQueue) Stop(ctx context.Context) error  { return nil }
func (mq *mockQueue) Register(name string, fn func(context.Context, []datamodel.Link) error, opts ...worker.JobOption[[]datamodel.Link]) error {
	// registration happens at constructions, kinda gross, ohh weell.
	return nil
}
func (mq *mockQueue) Enqueue(ctx context.Context, name string, msg []datamodel.Link) error {
	mq.enqueuedCount.Add(1)

	// Simulate processing delay if configured
	if mq.delay > 0 {
		time.Sleep(mq.delay)
	}

	// Simulate failures if configured
	if mq.failRate > 0 && mq.failRate > float32(time.Now().UnixNano()%100)/100 {
		return fmt.Errorf("simulated queue failure")
	}

	return mq.taskHandler.Handle(ctx, msg)
}

func newBufferStore(t *testing.T) aggregator.BufferStore {
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	buf, err := aggregator.NewSubmissionWorkspace(aggregator.SubmissionWorkspaceParams{
		Datastore: ds,
	})
	require.NoError(t, err)
	return buf
}

type fakeTaskHandler struct {
	called         atomic.Int64
	totalLinks     atomic.Int64
	mu             sync.Mutex
	processedLinks []datamodel.Link // Track all processed links
	delay          time.Duration    // Simulated processing delay
}

func (f *fakeTaskHandler) Handle(ctx context.Context, links []datamodel.Link) error {
	f.called.Add(1)
	f.totalLinks.Add(int64(len(links)))

	// Track processed links if needed
	if f.processedLinks != nil {
		f.mu.Lock()
		f.processedLinks = append(f.processedLinks, links...)
		f.mu.Unlock()
	}

	// Simulate processing delay if configured
	if f.delay > 0 {
		time.Sleep(f.delay)
	}

	return nil
}

// setupTestManager creates a test manager with mocked dependencies
func setupTestManager(t *testing.T, opts ...aggregator.ManagerOption) (*aggregator.Manager, aggregator.BufferStore, *fakeTaskHandler) {
	t.Helper()

	// Create real buffer store
	bufferStore := newBufferStore(t)

	// Create a simple test task handler that doesn't do anything by default
	// Individual tests can override this if they need specific behavior
	taskHandler := &fakeTaskHandler{}

	// Create a mock queue for testing
	queue := &mockQueue{taskHandler: taskHandler}

	// Create test app with fx for lifecycle management
	var manager *aggregator.Manager
	app := fxtest.New(t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				queue,
				fx.As(new(jobqueue.Service[[]datamodel.Link])),
			),
		),
		fx.Provide(func() aggregator.TaskHandler {
			return taskHandler
		}),
		fx.Provide(func() aggregator.BufferStore {
			return bufferStore
		}),
		fx.Supply(opts),
		fx.Provide(aggregator.NewManager),
		fx.Populate(&manager),
	)

	app.RequireStart()
	t.Cleanup(func() {
		app.RequireStop()
	})

	return manager, bufferStore, taskHandler
}

// TestManagerInitialization tests the manager initialization
func TestManagerInitialization(t *testing.T) {
	manager, buffer, handler := setupTestManager(t)
	require.NotNil(t, manager)
	require.NotNil(t, buffer)
	require.Equal(t, int64(0), handler.called.Load())
}

// TestManager_Submit tests the Submit method
func TestManagerSubmit(t *testing.T) {
	t.Run("single link no task spawned", func(t *testing.T) {
		manager, buffer, handler := setupTestManager(t)

		link := testutil.RandomCID(t)
		err := manager.Submit(t.Context(), link)
		require.NoError(t, err)

		aggs, err := buffer.Aggregates(t.Context())
		require.NoError(t, err)
		require.Len(t, aggs.Pending, 1)
		require.Equal(t, int64(0), handler.called.Load())
	})

	t.Run("single link task spawned after poll interval", func(t *testing.T) {
		tClock := clock.NewMock()
		manager, buffer, handler := setupTestManager(t, aggregator.WithClock(tClock))

		link := testutil.RandomCID(t)
		err := manager.Submit(t.Context(), link)
		require.NoError(t, err)

		aggs, err := buffer.Aggregates(t.Context())
		require.NoError(t, err)
		require.Len(t, aggs.Pending, 1)
		require.Equal(t, int64(0), handler.called.Load())

		// advance clock one poll interval
		tClock.Add(aggregator.DefaultPollInterval)
		aggs, err = buffer.Aggregates(t.Context())
		require.NoError(t, err)
		require.Len(t, aggs.Pending, 0)
		require.Equal(t, int64(1), handler.called.Load())
	})

	t.Run("single link task spawned after max size reached", func(t *testing.T) {
		tClock := clock.NewMock()
		batchSize := 3

		manager, buffer, handler := setupTestManager(t, aggregator.WithMaxBatchSize(batchSize), aggregator.WithClock(tClock))

		// add a batch size
		for i := 1; i < batchSize+1; i++ {
			link := testutil.RandomCID(t)
			err := manager.Submit(t.Context(), link)
			require.NoError(t, err)

			aggs, err := buffer.Aggregates(t.Context())
			require.NoError(t, err)
			require.Len(t, aggs.Pending, i)
			require.Equal(t, int64(0), handler.called.Load())
		}

		// add one more link, for submission
		link := testutil.RandomCID(t)
		err := manager.Submit(t.Context(), link)
		require.NoError(t, err)

		aggs, err := buffer.Aggregates(t.Context())
		require.NoError(t, err)
		require.Len(t, aggs.Pending, 1)
		require.Equal(t, int64(1), handler.called.Load())

		// advance clock, should spawn task
		tClock.Add(aggregator.DefaultPollInterval)

		aggs, err = buffer.Aggregates(t.Context())
		require.NoError(t, err)
		require.Len(t, aggs.Pending, 0)
		require.Equal(t, int64(2), handler.called.Load())
	})

	t.Run("large input exceeding batch size is properly batched", func(t *testing.T) {
		tClock := clock.NewMock()
		batchSize := 10

		manager, buffer, handler := setupTestManager(t, aggregator.WithMaxBatchSize(batchSize), aggregator.WithClock(tClock))

		// First, add some links to partially fill the buffer
		initialLinks := 3
		for i := 0; i < initialLinks; i++ {
			link := testutil.RandomCID(t)
			err := manager.Submit(t.Context(), link)
			require.NoError(t, err)
		}

		// Verify buffer has 3 links
		aggs, err := buffer.Aggregates(t.Context())
		require.NoError(t, err)
		require.Len(t, aggs.Pending, initialLinks)
		require.Equal(t, int64(0), handler.called.Load())

		// Now submit a large batch that exceeds max size (25 links)
		// This should trigger:
		// 1. Submit current buffer (3 links)
		// 2. Submit 2 full batches (10 links each = 20 links)
		// 3. Buffer the remaining 5 links
		largeInput := make([]datamodel.Link, 25)
		for i := 0; i < 25; i++ {
			largeInput[i] = testutil.RandomCID(t)
		}

		err = manager.Submit(t.Context(), largeInput...)
		require.NoError(t, err)

		// Verify:
		// - 3 batches were submitted (3 + 10 + 10 = 23 links)
		// - 5 links remain in buffer
		aggs, err = buffer.Aggregates(t.Context())
		require.NoError(t, err)
		require.Len(t, aggs.Pending, 5, "Should have 5 remaining links in buffer")
		require.Equal(t, int64(3), handler.called.Load(), "Should have submitted 3 batches")
		require.Equal(t, int64(23), handler.totalLinks.Load(), "Should have processed 23 links total")
	})

}

// TestManagerParallelSubmit tests concurrent Submit operations
func TestManagerParallelSubmit(t *testing.T) {
	t.Run("10 concurrent submits", func(t *testing.T) {
		// Use a large batch size to prevent immediate submissions
		maxBatchSize := 500 // Can hold ~2000 links
		manager, buffer, handler := setupTestManager(t, aggregator.WithMaxBatchSize(maxBatchSize))

		numGoroutines := 10
		linksPerGoroutine := maxBatchSize
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		// Track submission counts
		submittedCount := atomic.Int64{}
		errorCount := atomic.Int64{}

		// Spawn concurrent goroutines
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < linksPerGoroutine; j++ {
					link := testutil.RandomCID(t)
					err := manager.Submit(context.Background(), link)
					if err != nil {
						errorCount.Add(1)
						t.Logf("goroutine %d failed to submit link %d: %v", id, j, err)
					} else {
						submittedCount.Add(1)
					}
				}
			}(i)
		}

		wg.Wait()

		// Verify no errors
		require.Equal(t, int64(0), errorCount.Load(), "Got %d errors during submission", errorCount.Load())

		// Verify all links are in buffer or processed
		aggs, err := buffer.Aggregates(context.Background())
		require.NoError(t, err)

		// Total links should be either in buffer or processed
		totalExpected := numGoroutines * linksPerGoroutine
		totalProcessed := handler.totalLinks.Load()
		totalInBuffer := len(aggs.Pending)

		require.Equal(t, int64(totalExpected), totalProcessed+int64(totalInBuffer),
			"Total links mismatch: expected %d, got %d processed + %d in buffer",
			totalExpected, totalProcessed, totalInBuffer)

		// Most should be in buffer since batch size is large
		require.Greater(t, totalInBuffer, 0, "Should have links in buffer with large batch size")
	})

	t.Run("submit while processLoop is submitting", func(t *testing.T) {
		tClock := clock.NewMock()
		manager, buffer, handler := setupTestManager(t,
			aggregator.WithClock(tClock),
			aggregator.WithPollInterval(100*time.Millisecond))

		// Add initial links
		for i := 0; i < 5; i++ {
			err := manager.Submit(context.Background(), testutil.RandomCID(t))
			require.NoError(t, err)
		}

		// Start concurrent submissions
		var wg sync.WaitGroup
		wg.Add(2)

		// Goroutine 1: Trigger processLoop by advancing clock
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)  // Small delay
			tClock.Add(100 * time.Millisecond) // Trigger processLoop
		}()

		// Goroutine 2: Submit while processLoop might be running
		go func() {
			defer wg.Done()
			time.Sleep(15 * time.Millisecond) // Submit during processLoop
			for i := 0; i < 10; i++ {
				err := manager.Submit(context.Background(), testutil.RandomCID(t))
				require.NoError(t, err)
			}
		}()

		wg.Wait()

		// Give processLoop time to complete
		time.Sleep(100 * time.Millisecond)

		// Verify data integrity
		aggs, err := buffer.Aggregates(context.Background())
		require.NoError(t, err)

		totalInBuffer := len(aggs.Pending)
		totalProcessed := handler.totalLinks.Load()

		// Should have processed some and maybe some in buffer
		require.GreaterOrEqual(t, totalProcessed+int64(totalInBuffer), int64(5),
			"Should have at least the initial 5 links")
	})
}

// TestManagerShutdownUnderLoad tests graceful shutdown while under load
func TestManagerShutdownUnderLoad(t *testing.T) {
	t.Run("stop while submitting", func(t *testing.T) {
		tClock := clock.NewMock()
		manager, buffer, handler := setupTestManager(t,
			aggregator.WithClock(tClock),
			aggregator.WithPollInterval(50*time.Millisecond))

		// Start continuous submissions
		stopSubmitting := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(10)

		for i := 0; i < 10; i++ {
			go func(id int) {
				defer wg.Done()
				for {
					select {
					case <-stopSubmitting:
						return
					default:
						link := testutil.RandomCID(t)
						_ = manager.Submit(context.Background(), link)
						time.Sleep(time.Millisecond)
					}
				}
			}(i)
		}

		// Let it run for a bit
		time.Sleep(100 * time.Millisecond)

		// Stop submissions
		close(stopSubmitting)
		wg.Wait()

		// Trigger a final poll before shutdown
		tClock.Add(50 * time.Millisecond)
		time.Sleep(50 * time.Millisecond)

		// Record state before shutdown
		aggs, err := buffer.Aggregates(context.Background())
		require.NoError(t, err)
		linksBeforeStop := len(aggs.Pending)
		processedBeforeStop := handler.totalLinks.Load()

		// Stop the manager
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = manager.Stop(ctx)
		require.NoError(t, err)

		// Verify no data loss after shutdown
		aggs, err = buffer.Aggregates(context.Background())
		require.NoError(t, err)

		// Buffer should still have unprocessed links or they should be processed
		totalAfterStop := handler.totalLinks.Load() + int64(len(aggs.Pending))
		totalBeforeStop := processedBeforeStop + int64(linksBeforeStop)

		require.GreaterOrEqual(t, totalAfterStop, totalBeforeStop,
			"Should not lose data during shutdown")
	})
}

// TestManagerSustainedLoadPatterns tests different load patterns
func TestManagerSustainedLoadPatterns(t *testing.T) {
	t.Run("burst load pattern", func(t *testing.T) {
		tClock := clock.NewMock()
		manager, buffer, handler := setupTestManager(t,
			aggregator.WithClock(tClock),
			aggregator.WithPollInterval(100*time.Millisecond),
			aggregator.WithMaxBatchSize(1024))

		// Burst phase: submit many links quickly
		burstSize := 500
		for i := 0; i < burstSize; i++ {
			err := manager.Submit(context.Background(), testutil.RandomCID(t))
			require.NoError(t, err)
		}

		// Check buffer filled during burst
		aggs, err := buffer.Aggregates(context.Background())
		require.NoError(t, err)
		require.Greater(t, len(aggs.Pending), 0, "Buffer should have links after burst")

		// Trigger processing
		tClock.Add(100 * time.Millisecond)
		time.Sleep(50 * time.Millisecond) // Give processLoop time to run

		// Verify burst was processed
		aggs, err = buffer.Aggregates(context.Background())
		require.NoError(t, err)

		totalProcessed := handler.totalLinks.Load()
		totalInBuffer := len(aggs.Pending)

		require.Equal(t, int64(burstSize), totalProcessed+int64(totalInBuffer),
			"All burst links should be accounted for")
	})

	t.Run("steady load pattern", func(t *testing.T) {
		tClock := clock.NewMock()
		pollInterval := 50 * time.Millisecond
		manager, _, handler := setupTestManager(t,
			aggregator.WithClock(tClock),
			aggregator.WithPollInterval(pollInterval),
			aggregator.WithMaxBatchSize(512))

		// Simulate steady load over multiple poll intervals
		stop := make(chan struct{})
		go func() {
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					_ = manager.Submit(context.Background(), testutil.RandomCID(t))
				}
			}
		}()

		// Run for several poll intervals
		for i := 0; i < 5; i++ {
			time.Sleep(20 * time.Millisecond)
			tClock.Add(pollInterval)
		}

		close(stop)
		time.Sleep(50 * time.Millisecond) // Let final processing complete

		// Should have multiple batches processed
		require.Greater(t, handler.called.Load(), int64(2), "Should have processed multiple batches under steady load")
		require.Greater(t, handler.totalLinks.Load(), int64(0), "Should have processed links")
	})

	t.Run("variable load pattern", func(t *testing.T) {
		tClock := clock.NewMock()
		manager, _, handler := setupTestManager(t,
			aggregator.WithClock(tClock),
			aggregator.WithPollInterval(50*time.Millisecond))

		// High load phase
		for i := 0; i < 100; i++ {
			err := manager.Submit(context.Background(), testutil.RandomCID(t))
			require.NoError(t, err)
		}

		// Process high load
		tClock.Add(50 * time.Millisecond)
		time.Sleep(20 * time.Millisecond)

		highLoadCalls := handler.called.Load()

		// Low load phase
		for i := 0; i < 10; i++ {
			err := manager.Submit(context.Background(), testutil.RandomCID(t))
			require.NoError(t, err)
			time.Sleep(time.Millisecond)
		}

		// Process low load
		tClock.Add(50 * time.Millisecond)
		time.Sleep(20 * time.Millisecond)

		// Another high load phase
		for i := 0; i < 100; i++ {
			err := manager.Submit(context.Background(), testutil.RandomCID(t))
			require.NoError(t, err)
		}

		// Final processing
		tClock.Add(50 * time.Millisecond)
		time.Sleep(20 * time.Millisecond)

		// Should have handled variable load appropriately
		totalCalls := handler.called.Load()
		require.Greater(t, totalCalls, highLoadCalls, "Should have processed additional batches")
		require.Equal(t, int64(210), handler.totalLinks.Load(), "Should have processed all 210 links")
	})
}

// TestManagerLongRunningStress tests for memory leaks and stability over time
func TestManagerLongRunningStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running stress test in short mode")
	}

	t.Run("extended continuous load", func(t *testing.T) {
		tClock := clock.NewMock()
		manager, buffer, handler := setupTestManager(t,
			aggregator.WithClock(tClock),
			aggregator.WithPollInterval(50*time.Millisecond),
			aggregator.WithMaxBatchSize(2048))

		// Track initial goroutine count
		initialGoroutines := runtime.NumGoroutine()

		// Run continuous load for extended period
		testDuration := 10 * time.Second
		stop := make(chan struct{})
		submissionCount := atomic.Int64{}

		// Start multiple submission goroutines
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
						link := testutil.RandomCID(t)
						if err := manager.Submit(context.Background(), link); err == nil {
							submissionCount.Add(1)
						}
						// Vary submission rate
						if id%2 == 0 {
							time.Sleep(time.Microsecond * 100)
						}
					}
				}
			}(i)
		}

		// Simulate time passing and trigger periodic processing
		startTime := time.Now()
		_ = startTime
		processingTicker := time.NewTicker(100 * time.Millisecond)
		defer processingTicker.Stop()

		go func() {
			for {
				select {
				case <-stop:
					return
				case <-processingTicker.C:
					tClock.Add(50 * time.Millisecond)
				}
			}
		}()

		// Let it run
		time.Sleep(testDuration)

		// Stop submissions
		close(stop)
		wg.Wait()

		// Final processing
		tClock.Add(50 * time.Millisecond)
		time.Sleep(100 * time.Millisecond)

		// Check for goroutine leaks
		time.Sleep(100 * time.Millisecond) // Give goroutines time to clean up
		finalGoroutines := runtime.NumGoroutine()
		goroutineDiff := finalGoroutines - initialGoroutines

		// Allow some tolerance for test framework goroutines
		require.LessOrEqual(t, goroutineDiff, 5,
			"Possible goroutine leak detected: started with %d, ended with %d goroutines",
			initialGoroutines, finalGoroutines)

		// Verify data integrity
		aggs, err := buffer.Aggregates(context.Background())
		require.NoError(t, err)

		totalSubmitted := submissionCount.Load()
		totalProcessed := handler.totalLinks.Load()
		totalInBuffer := int64(len(aggs.Pending))

		require.Equal(t, totalSubmitted, totalProcessed+totalInBuffer,
			"Data integrity check failed: submitted %d, processed %d, in buffer %d",
			totalSubmitted, totalProcessed, totalInBuffer)

		// Should have processed many batches
		require.Greater(t, handler.called.Load(), int64(10),
			"Should have processed many batches during long run")

		t.Logf("Long-running test completed: %d submissions, %d batches processed, %d links processed",
			totalSubmitted, handler.called.Load(), totalProcessed)
	})
}
