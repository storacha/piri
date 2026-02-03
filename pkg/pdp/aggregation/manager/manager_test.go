package manager_test

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
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/storacha/piri/lib/jobqueue"
	"github.com/storacha/piri/lib/jobqueue/worker"
	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/pdp/aggregation/manager"
)

// mockConfigProvider implements manager.ConfigProvider for testing.
// Tests must set pollInterval and batchSize explicitly - there are no defaults.
// This mock supports dynamic config changes via SetPollInterval and SetBatchSize methods.
type mockConfigProvider struct {
	mu           sync.RWMutex
	pollInterval time.Duration
	batchSize    uint
	workers      uint
	subscribers  map[config.Key][]func(old, new any)
}

func (m *mockConfigProvider) PollInterval() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pollInterval
}

func (m *mockConfigProvider) BatchSize() uint {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.batchSize
}

func (m *mockConfigProvider) Workers() uint {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.workers
}

func (m *mockConfigProvider) Subscribe(key config.Key, fn func(old, new any)) (func(), error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.subscribers == nil {
		m.subscribers = make(map[config.Key][]func(old, new any))
	}
	m.subscribers[key] = append(m.subscribers[key], fn)
	return func() {
		// Unsubscribe - not strictly needed for tests but good for completeness
	}, nil
}

// SetPollInterval changes the poll interval and notifies subscribers.
func (m *mockConfigProvider) SetPollInterval(d time.Duration) {
	m.mu.Lock()
	old := m.pollInterval
	m.pollInterval = d
	callbacks := append([]func(old, new any){}, m.subscribers[config.ManagerPollInterval]...)
	m.mu.Unlock()

	// Call callbacks outside lock to avoid deadlock
	for _, fn := range callbacks {
		fn(old, d)
	}
}

// SetBatchSize changes the batch size. No subscription mechanism - read on demand.
func (m *mockConfigProvider) SetBatchSize(size uint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.batchSize = size
}

// mockQueue is a simple implementation of jobqueue.Service for testing
type mockQueue struct {
	taskHandler   jobqueue.TaskHandler[[]datamodel.Link]
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
func (mq *mockQueue) RegisterHandler(h jobqueue.TaskHandler[[]datamodel.Link], opts ...worker.JobOption[[]datamodel.Link]) error {
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

func newBufferStore(t *testing.T) manager.BufferStore {
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	buf, err := manager.NewSubmissionWorkspace(manager.SubmissionWorkspaceParams{
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

func (f *fakeTaskHandler) Name() string {
	return "fakeTaskHandler"
}

// setupTestManager creates a test manager with mocked dependencies
func setupTestManager(t *testing.T, cfgProvider *mockConfigProvider, opts ...manager.ManagerOption) (*manager.Manager, manager.BufferStore, *fakeTaskHandler) {
	t.Helper()

	// Create real buffer store
	bufferStore := newBufferStore(t)

	// Create a simple test task handler that doesn't do anything by default
	// Individual tests can override this if they need specific behavior
	taskHandler := &fakeTaskHandler{}

	// Create a mock queue for testing
	queue := &mockQueue{taskHandler: taskHandler}

	// Build option providers with proper group annotations for fx
	optProviders := make([]fx.Option, 0, len(opts))
	for _, opt := range opts {
		o := opt // capture loop variable
		optProviders = append(optProviders, fx.Provide(
			fx.Annotate(
				func() manager.ManagerOption { return o },
				fx.ResultTags(`group:"manager_options"`),
			),
		))
	}

	// Create test app with fx for lifecycle management
	var m *manager.Manager
	app := fxtest.New(t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				queue,
				fx.As(new(jobqueue.Service[[]datamodel.Link])),
			),
		),
		fx.Provide(func() jobqueue.TaskHandler[[]datamodel.Link] {
			return taskHandler
		}),
		fx.Provide(func() manager.BufferStore {
			return bufferStore
		}),
		fx.Provide(func() manager.ConfigProvider {
			return cfgProvider
		}),
		fx.Options(optProviders...),
		fx.Provide(manager.NewManager),
		fx.Populate(&m),
	)

	app.RequireStart()
	t.Cleanup(func() {
		app.RequireStop()
	})

	return m, bufferStore, taskHandler
}

// TestManagerInitialization tests the manager initialization
func TestManagerInitialization(t *testing.T) {
	mgr, buffer, handler := setupTestManager(t, &mockConfigProvider{
		pollInterval: manager.DefaultPollInterval,
		batchSize:    manager.DefaultMaxBatchSizeBytes,
	})
	require.NotNil(t, mgr)
	require.NotNil(t, buffer)
	require.Equal(t, int64(0), handler.called.Load())
}

// TestManager_Submit tests the Submit method
func TestManagerSubmit(t *testing.T) {
	t.Run("single link no task spawned", func(t *testing.T) {
		mgr, buffer, handler := setupTestManager(t, &mockConfigProvider{
			pollInterval: manager.DefaultPollInterval,
			batchSize:    manager.DefaultMaxBatchSizeBytes,
		})

		link := testutil.RandomCID(t)
		err := mgr.Submit(t.Context(), link)
		require.NoError(t, err)

		aggs, err := buffer.Aggregation(t.Context())
		require.NoError(t, err)
		require.Len(t, aggs.Roots, 1)
		require.Equal(t, int64(0), handler.called.Load())
	})

	t.Run("single link task spawned after poll interval", func(t *testing.T) {
		tClock := clock.NewMock()
		m, buffer, handler := setupTestManager(t, &mockConfigProvider{
			pollInterval: manager.DefaultPollInterval,
			batchSize:    manager.DefaultMaxBatchSizeBytes,
		}, manager.WithClock(tClock))

		link := testutil.RandomCID(t)
		err := m.Submit(t.Context(), link)
		require.NoError(t, err)

		aggs, err := buffer.Aggregation(t.Context())
		require.NoError(t, err)
		require.Len(t, aggs.Roots, 1)
		require.Equal(t, int64(0), handler.called.Load())

		// cleaning up buffer is async, so we expect it to happen sometime soonish
		require.Eventually(t, func() bool {
			// advance clock one poll interval
			// NB(forrest): we do this internally to ensure the ticker fires before checking
			tClock.Add(manager.DefaultPollInterval + 1)

			aggs, err = buffer.Aggregation(t.Context())
			t.Logf("waiting on %d aggregats to clearn", len(aggs.Roots))
			require.NoError(t, err)
			return len(aggs.Roots) == 0
		}, 30*time.Second, time.Millisecond)
	})

	t.Run("single link task spawned after max size reached", func(t *testing.T) {
		tClock := clock.NewMock()
		batchSize := uint(3)

		m, buffer, handler := setupTestManager(t, &mockConfigProvider{
			pollInterval: manager.DefaultPollInterval,
			batchSize:    batchSize,
		}, manager.WithClock(tClock))

		// add a batch size
		for i := 1; i < int(batchSize)+1; i++ {
			link := testutil.RandomCID(t)
			err := m.Submit(t.Context(), link)
			require.NoError(t, err)

			aggs, err := buffer.Aggregation(t.Context())
			require.NoError(t, err)
			require.Len(t, aggs.Roots, i)
			require.Equal(t, int64(0), handler.called.Load())
		}

		// add one more link, for submission
		link := testutil.RandomCID(t)
		err := m.Submit(t.Context(), link)
		require.NoError(t, err)

		aggs, err := buffer.Aggregation(t.Context())
		require.NoError(t, err)
		require.Len(t, aggs.Roots, 1)
		require.Equal(t, int64(1), handler.called.Load())

		// cleaning up buffer is async, so we expect it to happen sometime soonish
		require.Eventually(t, func() bool {
			// advance clock one poll interval
			// NB(forrest): we do this internally to ensure the ticker fires before checking
			tClock.Add(manager.DefaultPollInterval)

			aggs, err = buffer.Aggregation(t.Context())
			require.NoError(t, err)
			return len(aggs.Roots) == 0
		}, 3*time.Second, 500*time.Millisecond)
	})

	t.Run("large input exceeding batch size is properly batched", func(t *testing.T) {
		tClock := clock.NewMock()
		batchSize := uint(10)

		m, buffer, handler := setupTestManager(t, &mockConfigProvider{
			pollInterval: manager.DefaultPollInterval,
			batchSize:    batchSize,
		}, manager.WithClock(tClock))

		// First, add some links to partially fill the buffer
		initialLinks := 3
		for i := 0; i < initialLinks; i++ {
			link := testutil.RandomCID(t)
			err := m.Submit(t.Context(), link)
			require.NoError(t, err)
		}

		// Verify buffer has 3 links
		aggs, err := buffer.Aggregation(t.Context())
		require.NoError(t, err)
		require.Len(t, aggs.Roots, initialLinks)
		require.Equal(t, int64(0), handler.called.Load())

		// Now submit a large batch that exceeds max size (25 links)
		// With optimized batching, this should trigger:
		// 1. Fill current buffer (3 links) to max by adding 7 from new links, submit full batch (10 links)
		// 2. Submit 1 more full batch (10 links) from remaining 18 links
		// 3. Buffer the remaining 8 links
		largeInput := make([]datamodel.Link, 25)
		for i := 0; i < 25; i++ {
			largeInput[i] = testutil.RandomCID(t)
		}

		err = m.Submit(t.Context(), largeInput...)
		require.NoError(t, err)

		// Verify:
		// - 2 batches were submitted (10 + 10 = 20 links)
		// - 8 links remain in buffer (3 initial + 25 new - 20 submitted = 8)
		aggs, err = buffer.Aggregation(t.Context())
		require.NoError(t, err)
		require.Len(t, aggs.Roots, 8, "Should have 8 remaining links in buffer")
		require.Equal(t, int64(2), handler.called.Load(), "Should have submitted 2 batches")
		require.Equal(t, int64(20), handler.totalLinks.Load(), "Should have processed 20 links total")
	})

}

// TestManagerParallelSubmit tests concurrent Submit operations
func TestManagerParallelSubmit(t *testing.T) {
	t.Run("10 concurrent submits", func(t *testing.T) {
		// Use a large batch size to prevent immediate submissions
		maxBatchSize := uint(500) // Can hold ~2000 links
		m, buffer, handler := setupTestManager(t, &mockConfigProvider{
			pollInterval: manager.DefaultPollInterval,
			batchSize:    maxBatchSize,
		})

		numGoroutines := 10
		linksPerGoroutine := int(maxBatchSize)
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
					err := m.Submit(context.Background(), link)
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
		aggs, err := buffer.Aggregation(context.Background())
		require.NoError(t, err)

		// Total links should be either in buffer or processed
		totalExpected := numGoroutines * linksPerGoroutine
		totalProcessed := handler.totalLinks.Load()
		totalInBuffer := len(aggs.Roots)

		require.Equal(t, int64(totalExpected), totalProcessed+int64(totalInBuffer),
			"Total links mismatch: expected %d, got %d processed + %d in buffer",
			totalExpected, totalProcessed, totalInBuffer)

		// Most should be in buffer since batch size is large
		require.Greater(t, totalInBuffer, 0, "Should have links in buffer with large batch size")
	})

	t.Run("submit while processLoop is submitting", func(t *testing.T) {
		tClock := clock.NewMock()
		m, buffer, handler := setupTestManager(t, &mockConfigProvider{
			pollInterval: 100 * time.Millisecond,
			batchSize:    manager.DefaultMaxBatchSizeBytes,
		}, manager.WithClock(tClock))

		// Add initial links
		for i := 0; i < 5; i++ {
			err := m.Submit(context.Background(), testutil.RandomCID(t))
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
				err := m.Submit(context.Background(), testutil.RandomCID(t))
				require.NoError(t, err)
			}
		}()

		wg.Wait()

		// Give processLoop time to complete
		time.Sleep(100 * time.Millisecond)

		// Verify data integrity
		aggs, err := buffer.Aggregation(context.Background())
		require.NoError(t, err)

		totalInBuffer := len(aggs.Roots)
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
		m, buffer, handler := setupTestManager(t, &mockConfigProvider{
			pollInterval: 50 * time.Millisecond,
			batchSize:    manager.DefaultMaxBatchSizeBytes,
		}, manager.WithClock(tClock))

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
						_ = m.Submit(context.Background(), link)
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
		aggs, err := buffer.Aggregation(context.Background())
		require.NoError(t, err)
		linksBeforeStop := len(aggs.Roots)
		processedBeforeStop := handler.totalLinks.Load()

		// Stop the manager
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = m.Stop(ctx)
		require.NoError(t, err)

		// Verify no data loss after shutdown
		aggs, err = buffer.Aggregation(context.Background())
		require.NoError(t, err)

		// Buffer should still have unprocessed links or they should be processed
		totalAfterStop := handler.totalLinks.Load() + int64(len(aggs.Roots))
		totalBeforeStop := processedBeforeStop + int64(linksBeforeStop)

		require.GreaterOrEqual(t, totalAfterStop, totalBeforeStop,
			"Should not lose data during shutdown")
	})
}

// TestManagerSustainedLoadPatterns tests different load patterns
func TestManagerSustainedLoadPatterns(t *testing.T) {
	t.Run("burst load pattern", func(t *testing.T) {
		tClock := clock.NewMock()
		m, buffer, handler := setupTestManager(t, &mockConfigProvider{
			pollInterval: 100 * time.Millisecond,
			batchSize:    1024,
		}, manager.WithClock(tClock))

		// Burst phase: submit many links quickly
		burstSize := 500
		for i := 0; i < burstSize; i++ {
			err := m.Submit(context.Background(), testutil.RandomCID(t))
			require.NoError(t, err)
		}

		// Check buffer filled during burst
		aggs, err := buffer.Aggregation(context.Background())
		require.NoError(t, err)
		require.Greater(t, len(aggs.Roots), 0, "Buffer should have links after burst")

		// Trigger processing
		tClock.Add(100 * time.Millisecond)
		time.Sleep(50 * time.Millisecond) // Give processLoop time to run

		// Verify burst was processed
		aggs, err = buffer.Aggregation(context.Background())
		require.NoError(t, err)

		totalProcessed := handler.totalLinks.Load()
		totalInBuffer := len(aggs.Roots)

		require.Equal(t, int64(burstSize), totalProcessed+int64(totalInBuffer),
			"All burst links should be accounted for")
	})

	t.Run("steady load pattern", func(t *testing.T) {
		tClock := clock.NewMock()
		pollInterval := 50 * time.Millisecond
		m, _, handler := setupTestManager(t, &mockConfigProvider{
			pollInterval: pollInterval,
			batchSize:    512,
		}, manager.WithClock(tClock))

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
					_ = m.Submit(context.Background(), testutil.RandomCID(t))
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
		m, _, handler := setupTestManager(t, &mockConfigProvider{
			pollInterval: 50 * time.Millisecond,
			batchSize:    manager.DefaultMaxBatchSizeBytes,
		}, manager.WithClock(tClock))

		// High load phase
		for i := 0; i < 100; i++ {
			err := m.Submit(context.Background(), testutil.RandomCID(t))
			require.NoError(t, err)
		}

		// Process high load
		tClock.Add(50 * time.Millisecond)
		time.Sleep(20 * time.Millisecond)

		highLoadCalls := handler.called.Load()

		// Low load phase
		for i := 0; i < 10; i++ {
			err := m.Submit(context.Background(), testutil.RandomCID(t))
			require.NoError(t, err)
			time.Sleep(time.Millisecond)
		}

		// Process low load
		tClock.Add(50 * time.Millisecond)
		time.Sleep(20 * time.Millisecond)

		// Another high load phase
		for i := 0; i < 100; i++ {
			err := m.Submit(context.Background(), testutil.RandomCID(t))
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
		m, buffer, handler := setupTestManager(t, &mockConfigProvider{
			pollInterval: 50 * time.Millisecond,
			batchSize:    2048,
		}, manager.WithClock(tClock))

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
						if err := m.Submit(context.Background(), link); err == nil {
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

		// Use separate wait group for clock goroutine to avoid data race
		var clockWg sync.WaitGroup
		clockWg.Add(1)
		go func() {
			defer clockWg.Done()
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
		clockWg.Wait() // Wait for clock goroutine to fully exit before calling tClock.Add()

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
		aggs, err := buffer.Aggregation(context.Background())
		require.NoError(t, err)

		totalSubmitted := submissionCount.Load()
		totalProcessed := handler.totalLinks.Load()
		totalInBuffer := int64(len(aggs.Roots))

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

// TestManagerDynamicConfig tests that the manager responds correctly to runtime
// configuration changes for poll interval and batch size.
func TestManagerDynamicConfig(t *testing.T) {
	t.Run("poll_interval_change_resets_ticker", func(t *testing.T) {
		tClock := clock.NewMock()
		cfgProvider := &mockConfigProvider{
			pollInterval: manager.DefaultPollInterval, // 30s
			batchSize:    manager.DefaultMaxBatchSizeBytes,
		}

		m, buffer, handler := setupTestManager(t, cfgProvider, manager.WithClock(tClock))

		// Submit a link to buffer
		link := testutil.RandomCID(t)
		err := m.Submit(t.Context(), link)
		require.NoError(t, err)

		// Verify link is in buffer
		aggs, err := buffer.Aggregation(t.Context())
		require.NoError(t, err)
		require.Len(t, aggs.Roots, 1)
		require.Equal(t, int64(0), handler.called.Load())

		// Change poll interval to 100ms (much shorter than 30s)
		cfgProvider.SetPollInterval(100 * time.Millisecond)

		// Give the manager time to process the config change signal
		time.Sleep(10 * time.Millisecond)

		// Advance clock by the NEW interval (100ms), not the old one (30s)
		// If ticker reset worked, this should trigger the processLoop
		require.Eventually(t, func() bool {
			tClock.Add(100 * time.Millisecond)
			aggs, err = buffer.Aggregation(t.Context())
			require.NoError(t, err)
			return len(aggs.Roots) == 0
		}, 5*time.Second, 10*time.Millisecond)

		// Verify submission occurred
		require.Equal(t, int64(1), handler.called.Load(), "Should have processed one batch after poll interval change")
	})

	t.Run("batch_size_decrease_triggers_submission", func(t *testing.T) {
		tClock := clock.NewMock()
		cfgProvider := &mockConfigProvider{
			pollInterval: manager.DefaultPollInterval,
			batchSize:    10, // Start with batch size of 10
		}

		m, buffer, handler := setupTestManager(t, cfgProvider, manager.WithClock(tClock))

		// Submit 5 items (under threshold of 10)
		for i := 0; i < 5; i++ {
			err := m.Submit(t.Context(), testutil.RandomCID(t))
			require.NoError(t, err)
		}

		// Verify 5 items in buffer, no submission yet
		aggs, err := buffer.Aggregation(t.Context())
		require.NoError(t, err)
		require.Len(t, aggs.Roots, 5)
		require.Equal(t, int64(0), handler.called.Load())

		// Decrease batch size to 3
		cfgProvider.SetBatchSize(3)

		// Submit 1 more item - now total is 6 which exceeds new batch size of 3
		// This triggers submission. The manager's behavior when buffer exceeds new limit:
		// - Current buffer (5 items) exceeds new max (3), so submit current buffer
		// - Then buffer the 1 new item
		err = m.Submit(t.Context(), testutil.RandomCID(t))
		require.NoError(t, err)

		// Verify submission occurred: current buffer of 5 was submitted, 1 new item buffered
		aggs, err = buffer.Aggregation(t.Context())
		require.NoError(t, err)

		require.Equal(t, int64(1), handler.called.Load(), "Should have submitted 1 batch (the existing buffer)")
		require.Equal(t, int64(5), handler.totalLinks.Load(), "Should have processed 5 links")
		require.Len(t, aggs.Roots, 1, "Should have 1 new item in buffer")
	})

	t.Run("batch_size_increase_delays_submission", func(t *testing.T) {
		tClock := clock.NewMock()
		cfgProvider := &mockConfigProvider{
			pollInterval: manager.DefaultPollInterval,
			batchSize:    3, // Start with small batch size
		}

		m, buffer, handler := setupTestManager(t, cfgProvider, manager.WithClock(tClock))

		// Submit 2 items (under threshold of 3)
		for i := 0; i < 2; i++ {
			err := m.Submit(t.Context(), testutil.RandomCID(t))
			require.NoError(t, err)
		}

		// Verify 2 items in buffer, no submission yet
		aggs, err := buffer.Aggregation(t.Context())
		require.NoError(t, err)
		require.Len(t, aggs.Roots, 2)
		require.Equal(t, int64(0), handler.called.Load())

		// Increase batch size to 10
		cfgProvider.SetBatchSize(10)

		// Submit 1 more item - total is 3, but new threshold is 10
		err = m.Submit(t.Context(), testutil.RandomCID(t))
		require.NoError(t, err)

		// Verify NO submission occurred - all 3 items still in buffer
		aggs, err = buffer.Aggregation(t.Context())
		require.NoError(t, err)
		require.Len(t, aggs.Roots, 3, "All items should still be in buffer")
		require.Equal(t, int64(0), handler.called.Load(), "No submission should have occurred")

		// Submit more items to reach the new threshold
		for i := 0; i < 7; i++ {
			err = m.Submit(t.Context(), testutil.RandomCID(t))
			require.NoError(t, err)
		}

		// Now we have 10 items, at the threshold - still no submission
		aggs, err = buffer.Aggregation(t.Context())
		require.NoError(t, err)
		require.Len(t, aggs.Roots, 10, "Should have 10 items in buffer at threshold")
		require.Equal(t, int64(0), handler.called.Load(), "No submission at exactly threshold")

		// Submit one more to exceed threshold
		err = m.Submit(t.Context(), testutil.RandomCID(t))
		require.NoError(t, err)

		// Now should have submitted: 10 items submitted, 1 remaining in buffer
		aggs, err = buffer.Aggregation(t.Context())
		require.NoError(t, err)
		require.Len(t, aggs.Roots, 1, "Should have 1 item remaining in buffer")
		require.Equal(t, int64(1), handler.called.Load(), "Should have submitted one batch")
		require.Equal(t, int64(10), handler.totalLinks.Load(), "Should have processed 10 links")
	})
}
