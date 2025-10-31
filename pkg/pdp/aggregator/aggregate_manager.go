package aggregator

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/raulk/clock"
	captypes "github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/serializer"
	"github.com/storacha/piri/pkg/pdp/types"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue"
)

// DefaultMaxBatchSizeBytes is the maximum size of batch.
const (
	// DefaultMaxBatchSizeBytes is the maximum number of pieces that may be submitted to be added as roots in a single operation
	DefaultMaxBatchSizeBytes = 10
	// DefaultPollInterval is the frequency the manager will flush its buffer to submit roots
	DefaultPollInterval = 30 * time.Second
	ManagerQueueName    = "manager"
	ManagerTaskName     = "add_roots"
)

var ManagerModule = fx.Module("aggregator/manager",
	fx.Provide(
		NewManager,
		NewSubmissionWorkspace,
		NewAddRootsTaskHandler,
		NewManagerQueue,
	),
)

type ManagerQueueParams struct {
	fx.In
	DB *sql.DB `name:"aggregator_db"`
}

func NewManagerQueue(params ManagerQueueParams) (jobqueue.Service[[]datamodel.Link], error) {
	managerQueue, err := jobqueue.New[[]datamodel.Link](
		ManagerQueueName,
		params.DB,
		&serializer.IPLDCBOR[[]datamodel.Link]{
			Typ:  bufferTS.TypeByName("AggregateLinks"),
			Opts: captypes.Converters,
		},
		jobqueue.WithLogger(log.With("queue", ManagerQueueName)),
		jobqueue.WithMaxRetries(50),
		// NB: must remain one to keep submissions serial to AddRoots
		jobqueue.WithMaxWorkers(uint(1)),
		// wait for twice a filecoin epoch to submit
		jobqueue.WithMaxTimeout(time.Minute),
	)
	if err != nil {
		return nil, fmt.Errorf("creating piece_link job-queue: %w", err)
	}
	// NB: queue lifecycle is handled by manager since it must register with queue before starting it
	return managerQueue, nil
}

// TaskHandler is a function that processes a batch of aggregate links
// It encapsulates the logic for how aggregates are submitted to the blockchain
type TaskHandler interface {
	Handle(ctx context.Context, links []datamodel.Link) error
}

// Manager handles batched submission of aggregates to the blockchain
type Manager struct {
	// input parameters
	taskHandler TaskHandler
	buffer      BufferStore
	queue       jobqueue.Service[[]datamodel.Link]

	// options
	pollInterval time.Duration
	maxBatchSize int
	clock        clock.Clock

	// locking
	submitMu sync.Mutex

	// Lifecycle management
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	running atomic.Bool
}

type ManagerParams struct {
	fx.In

	Queue       jobqueue.Service[[]datamodel.Link]
	TaskHandler TaskHandler
	Buffer      BufferStore
	Options     []ManagerOption `optional:"true"`
}

type ManagerOption func(*Manager)

func WithPollInterval(pollInterval time.Duration) ManagerOption {
	return func(mgr *Manager) {
		mgr.pollInterval = pollInterval
	}
}

func WithMaxBatchSize(maxBatchSize int) ManagerOption {
	return func(mgr *Manager) {
		mgr.maxBatchSize = maxBatchSize
	}
}

func WithClock(clock clock.Clock) ManagerOption {
	return func(mgr *Manager) {
		mgr.clock = clock
	}
}

// NewManager creates a new submission manager
func NewManager(lc fx.Lifecycle, params ManagerParams) (*Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		taskHandler: params.TaskHandler,
		buffer:      params.Buffer,
		queue:       params.Queue,

		// can override with options
		pollInterval: DefaultPollInterval,
		maxBatchSize: DefaultMaxBatchSizeBytes,
		clock:        clock.New(),

		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}

	for _, opt := range params.Options {
		opt(m)
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return m.Start()
		},
		OnStop: func(ctx context.Context) error {
			return m.Stop(ctx)
		},
	})

	return m, nil
}

// Submit adds aggregates to the buffer for submission
func (m *Manager) Submit(ctx context.Context, aggregateLinks ...datamodel.Link) error {
	if !m.running.Load() {
		return fmt.Errorf("manager is stopped")
	}
	m.submitMu.Lock()
	defer m.submitMu.Unlock()
	if len(aggregateLinks) == 0 {
		return nil
	}

	aggregates, err := m.buffer.Aggregation(ctx)
	if err != nil {
		return fmt.Errorf("getting buffer: %w", err)
	}

	currentSize := len(aggregates.Roots)
	newSize := currentSize + len(aggregateLinks)

	// If adding new aggregates would NOT exceed max, append and return
	if newSize <= m.maxBatchSize {
		log.Infow("Buffering aggregates for submission", "new_count", len(aggregateLinks), "new_size", newSize)
		if err := m.buffer.AppendRoots(ctx, aggregateLinks); err != nil {
			return fmt.Errorf("appending aggregates: %w", err)
		}
		return nil
	}

	// Buffer would overflow, optimize by filling to max size first
	log.Infow("Buffer would exceed max size, optimizing submission",
		"current_size", currentSize,
		"new_size", newSize,
		"max_size", m.maxBatchSize)

	// Calculate how many items we can add to reach max size
	itemsToAdd := m.maxBatchSize - currentSize

	// If current buffer has items, fill it to max and submit
	if currentSize > 0 && itemsToAdd > 0 {
		// Take items from new aggregates to fill buffer to max
		// But don't take more than we have available
		toTake := itemsToAdd
		if len(aggregateLinks) < toTake {
			toTake = len(aggregateLinks)
		}
		fillItems := aggregateLinks[:toTake]
		aggregateLinks = aggregateLinks[toTake:]

		log.Infow("Filling buffer to max size before submission",
			"adding", len(fillItems),
			"total", m.maxBatchSize)

		// Append to buffer to reach max size
		if err := m.buffer.AppendRoots(ctx, fillItems); err != nil {
			return fmt.Errorf("appending aggregates to fill buffer: %w", err)
		}

		// Get the full buffer and submit
		fullBuffer, err := m.buffer.Aggregation(ctx)
		if err != nil {
			return fmt.Errorf("getting full buffer: %w", err)
		}

		if err := m.doSubmit(fullBuffer); err != nil {
			return fmt.Errorf("submitting full buffer: %w", err)
		}
	} else if currentSize > 0 {
		// Current buffer has items but new aggregates alone exceed max, submit current buffer
		if err := m.doSubmit(aggregates); err != nil {
			return fmt.Errorf("submitting current buffer: %w", err)
		}
	}

	// Process remaining aggregates in batches
	remaining := aggregateLinks
	for len(remaining) > 0 {
		// Determine batch size
		batchSize := m.maxBatchSize
		if len(remaining) < batchSize {
			batchSize = len(remaining)
		}

		batch := remaining[:batchSize]
		remaining = remaining[batchSize:]

		// If this is a full batch, submit immediately
		if len(batch) == m.maxBatchSize {
			log.Infow("Submitting full batch of aggregates", "count", len(batch))
			if err := m.doSubmit(Aggregation{Roots: batch}); err != nil {
				return fmt.Errorf("submitting batch: %w", err)
			}
		} else {
			// Partial batch, add to buffer
			log.Infow("Buffering remaining aggregates", "count", len(batch))
			if err := m.buffer.AppendRoots(ctx, batch); err != nil {
				return fmt.Errorf("appending remaining aggregates: %w", err)
			}
		}
	}

	return nil
}

// Start begins background processing
func (m *Manager) Start() error {
	m.running.Store(true)
	log.Info("Starting submission manager")

	// Register the injected task handler with the queue
	if err := m.queue.Register(ManagerTaskName, m.taskHandler.Handle); err != nil {
		return fmt.Errorf("failed to register batch queue submit_roots task: %w", err)
	}

	// queue handles context internally
	if err := m.queue.Start(context.Background()); err != nil {
		return fmt.Errorf("failed to start batch queue: %w", err)
	}

	go m.processLoop()
	return nil
}

// Stop gracefully shuts down the manager
func (m *Manager) Stop(ctx context.Context) error {
	m.running.Store(false)
	log.Info("Stopping submission manager")
	// close processLoop, preventing new attempts to submit batches to queue
	m.cancel()

	// close the queue, further preventing new queued submissions from starting execution
	if err := m.queue.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop batch queue: %w", err)
	}

	// Wait for processLoop to exit
	select {
	case <-m.done:
		log.Info("Submission manager stopped cleanly")
	case <-ctx.Done():
		log.Warn("Submission manager stop timed out")
	}

	return nil
}

// processLoop runs the background submission loop
func (m *Manager) processLoop() {
	defer close(m.done)

	ticker := m.clock.Ticker(m.pollInterval)
	defer ticker.Stop()

	log.Infow("Starting submission process loop", "poll_interval", m.pollInterval)

	for {
		select {
		case <-m.ctx.Done():
			log.Info("Process loop exiting due to context cancellation")
			return

		case <-ticker.C:
			m.submitMu.Lock()
			aggregates, err := m.buffer.Aggregation(m.ctx)
			if err == nil {
				if err := m.doSubmit(aggregates); err != nil {
					log.Errorw("Process loop failed to process submission", "error", err)
				}
			} else {
				log.Errorw("Error getting buffered aggregates for submit", "error", err)
			}
			m.submitMu.Unlock()
		}
	}
}

// doSubmit tries to submit if there's work and no submission in progress
func (m *Manager) doSubmit(aggregates Aggregation) error {
	if len(aggregates.Roots) == 0 {
		// Nothing to submit, non-error: try again next pollInterval
		return nil
	}

	log.Infow("Starting aggregates batch submission", "count", len(aggregates.Roots))

	// TODO: we __really__ need enqueue and clear to be atomic, else we may re-enqueue
	// roots we have already queued if clear fails, which should be rare, but can result
	// in data we have already written to chain being written again, which will succeed,
	// and the customer (storacha) will pay for it twice - as punishment for its own code failure?
	// or 1. the add roots operation of the PDP service should check that the roots don't already exist
	// before submitting
	// or 2. the signing service should reject signing data that has already been added.
	// if either 1. or 2. are implemented, the task fill eventually leave the queue, moving to deadletter
	if err := m.queue.Enqueue(m.ctx, ManagerTaskName, aggregates.Roots); err != nil {
		return fmt.Errorf("failed to enqueue batch submission roots: %w", err)
	}

	// only clear the buffer if we successfully submit to our stateful job queue
	if err := m.buffer.ClearRoots(m.ctx); err != nil {
		return fmt.Errorf("failed to clear batch submission roots: %w", err)
	}

	return nil
}

// NewAddRootsTaskHandler creates a TaskHandler that submits aggregate roots to the blockchain
// This factory function encapsulates the API dependencies needed for the task
func NewAddRootsTaskHandler(
	api types.ProofSetAPI,
	proofSet ProofSetIDProvider,
	store AggregateStore,
) TaskHandler {
	return &AddRootsTaskHandler{
		api:      api,
		proofSet: proofSet,
		store:    store,
	}
}

type AddRootsTaskHandler struct {
	api      types.ProofSetAPI
	proofSet ProofSetIDProvider
	store    AggregateStore
}

func (a *AddRootsTaskHandler) Handle(ctx context.Context, links []datamodel.Link) error {
	proofSetID, err := a.proofSet.ProofSetID(ctx)
	if err != nil {
		return fmt.Errorf("getting proof set ID from proof set provider: %w", err)
	}

	// build the set of roots we will add
	// TODO we should be de-deduplicating roots here as is done in add_roots already of pdp service
	roots := make([]types.RootAdd, len(links))
	for i, aggregateLink := range links {
		// fetch each aggregate to submit
		a, err := a.store.Get(ctx, aggregateLink)
		if err != nil {
			return fmt.Errorf("reading aggregates: %w", err)
		}
		// record its root
		rootCID, err := cid.Decode(a.Root.Link().String())
		if err != nil {
			return fmt.Errorf("failed to decode aggregate root CID: %w", err)
		}
		// subroots
		subRoots := make([]cid.Cid, len(a.Pieces))
		for j, p := range a.Pieces {
			pcid, err := cid.Decode(p.Link.Link().String())
			if err != nil {
				return fmt.Errorf("failed to decode piece CID: %w", err)
			}
			subRoots[j] = pcid
		}
		roots[i] = types.RootAdd{
			Root:     rootCID,
			SubRoots: subRoots,
		}
	}

	txHash, err := a.api.AddRoots(ctx, proofSetID, roots)
	if err != nil {
		return fmt.Errorf("adding roots: %w", err)
	}
	log.Infow("added roots", "count", len(roots), "tx", txHash)
	return nil
}
