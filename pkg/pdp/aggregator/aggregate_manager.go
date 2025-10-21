package aggregator

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/ipld/go-ipld-prime/datamodel"
	types2 "github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/serializer"
	"go.uber.org/fx"
	"golang.org/x/sync/semaphore"

	"github.com/storacha/piri/pkg/pdp/types"
)

// Manager handles batched submission of aggregates to the blockchain
type Manager struct {
	// input parameters
	db       *sql.DB
	api      types.ProofSetAPI
	proofSet ProofSetIDProvider
	store    AggregateStore

	// internal fields
	buffer       BufferStore
	pollInterval time.Duration
	sem          *semaphore.Weighted                  // Controls concurrent submissions
	batchQueue   *jobqueue.JobQueue[[]datamodel.Link] // For queueing SuperAggregates

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	// Metrics
}

type ManagerParams struct {
	fx.In

	DB       *sql.DB `name:"aggregator_db"`
	Api      types.ProofSetAPI
	ProofSet ProofSetIDProvider
	Store    AggregateStore
	BufferDS datastore.Datastore `name:"aggregator_datastore"`
}

// NewManager creates a new submission manager
func NewManager(lc fx.Lifecycle, params ManagerParams) (*Manager, error) {
	batchQueue, err := jobqueue.New[[]datamodel.Link](
		"aggregate_batch",
		params.DB,
		&serializer.IPLDCBOR[[]datamodel.Link]{
			Typ:  bufferTS.TypeByName("AggregateLinks"),
			Opts: types2.Converters,
		},
		jobqueue.WithLogger(log.With("queue", "aggregate_batch")),
		jobqueue.WithMaxRetries(50),
		// one worker to keep things serial
		jobqueue.WithMaxWorkers(uint(1)),
		// one filecoin epoch since this is wrongly running tasks, we need yet another queue.....
		jobqueue.WithMaxTimeout(30*time.Second),
	)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		api:      params.Api,
		proofSet: params.ProofSet,
		store:    params.Store,

		buffer:       NewSubmissionWorkspace(store.SimpleStoreFromDatastore(namespace.Wrap(params.BufferDS, datastore.NewKey(ManagerKey)))),
		pollInterval: 30 * time.Second,
		sem:          semaphore.NewWeighted(1), // Allow only one concurrent submission
		batchQueue:   batchQueue,

		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return m.Start()
		},
		OnStop: func(context.Context) error {
			return m.Stop(ctx)
		},
	})

	return m, nil
}

// Submit adds aggregates to the buffer for submission
func (m *Manager) Submit(ctx context.Context, aggregateLinks []datamodel.Link) error {
	// TODO return an error if manager is shutting down
	if len(aggregateLinks) == 0 {
		return nil
	}

	log.Infow("Buffering aggregates for submission", "new_count", len(aggregateLinks))

	// Use atomic append operation
	if err := m.buffer.AppendAggregates(ctx, aggregateLinks); err != nil {
		return fmt.Errorf("appending aggregates: %w", err)
	}

	return nil
}

// Start begins background processing
func (m *Manager) Start() error {
	log.Info("Starting submission manager")

	if err := m.batchQueue.Register("submit_roots", func(ctx context.Context, links []datamodel.Link) error {
		proofSetID, err := m.proofSet.ProofSetID(ctx)
		if err != nil {
			return fmt.Errorf("getting proof set ID from proof set provider: %w", err)
		}
		// build the set of roots we will add
		// TODO we should be de-deduplicating roots here as is done in add_roots already of pdp service
		roots := make([]types.RootAdd, len(links))
		for i, aggregateLink := range links {
			// fetch each aggregate to submit
			a, err := m.store.Get(ctx, aggregateLink)
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
		txHash, err := m.api.AddRoots(ctx, proofSetID, roots)
		if err != nil {
			return fmt.Errorf("adding roots: %w", err)
		}
		log.Infow("added roots", "count", len(roots), "tx", txHash)
		return nil
	}); err != nil {
		return fmt.Errorf("failed to register batch queue submit_roots task: %w", err)
	}

	// queue handles context internally
	if err := m.batchQueue.Start(context.Background()); err != nil {
		return fmt.Errorf("failed to start batch queue: %w", err)
	}

	go m.processLoop()
	return nil
}

// Stop gracefully shuts down the manager
func (m *Manager) Stop(ctx context.Context) error {
	log.Info("Stopping submission manager")
	// close processLoop, preventing new attempts to submit batches to queue
	m.cancel()

	// close the batchQueue, further preventing new queued submissions from starting execution
	if err := m.batchQueue.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop batch queue: %w", err)
	}

	// TODO: consider if we can attempt to aquire semaphore and stop the queue in parallel.
	// The stop call above on the queue will block until active tasks complete.

	// TODO this feels optional? given above cancel and queue stoppage
	// Try to acquire semaphore to ensure no submission is in progress
	if err := m.sem.Acquire(ctx, 1); err == nil {
		// Got semaphore, no submission in progress
		m.sem.Release(1)
	} else {
		log.Warnw("Submission still in progress during shutdown", "error", err)
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

	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	log.Infow("Starting submission process loop", "poll_interval", m.pollInterval)

	for {
		select {
		case <-m.ctx.Done():
			log.Info("Process loop exiting due to context cancellation")
			return
		case <-ticker.C:
			if err := m.attemptSubmission(); err != nil {
				log.Errorw("Process loop failed to process submission", "error", err)
			}
		}
	}
}

// attemptSubmission tries to submit if there's work and no submission in progress
func (m *Manager) attemptSubmission() error {
	// Try to acquire semaphore (non-blocking)
	if !m.sem.TryAcquire(1) {
		// Submission already in progress, non-error: try again next pollInterval
		return nil
	}
	// we got a slot to submit, proceed
	defer m.sem.Release(1)

	// get the current buffered aggregates
	aggregates, err := m.buffer.Aggregates(m.ctx)
	if err != nil {
		return fmt.Errorf("failed to get aggregates: %w", err)
	}
	bufferSize := len(aggregates.Pending)

	if len(aggregates.Pending) == 0 {
		// Nothing to submit, non-error: try again next pollInterval
		return nil
	}

	// TODO: we __really__ need enqueue and clear to be atomic, else we may re-enqueue
	// roots we have already queued if clear fails, which should be rare, but can result
	// in data we have already written to chain being written again, which will succeed,
	// and the customer (storacha) will pay for it twice - as punishment for its own code failure?
	// or 1. the add roots operation of the PDP service should check that the roots don't already exist
	// before submitting
	// or 2. the signing service should reject signing data that has already been added.
	// if either 1. or 2. are implemented, the task fill eventually leave the queue, moving to deadletter
	log.Infow("Starting aggregates batch submission", "count", bufferSize)
	// enqueue buffered aggregates to submission to PDP service
	if err := m.batchQueue.Enqueue(m.ctx, "submit_roots", aggregates.Pending); err != nil {
		return fmt.Errorf("failed to enqueue batch submission roots: %w", err)
	}

	// only clear the buffer if we successfully submit to our stateful job queue
	if err := m.buffer.ClearAggregates(m.ctx); err != nil {
		return fmt.Errorf("failed to clear batch submission roots: %w", err)
	}

	return nil
}
