package jobqueue

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"

	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/queue"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/serializer"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/worker"
)

var log = logging.Logger("jobqueue")

type Service[T any] interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Register(name string, fn func(context.Context, T) error, opts ...worker.JobOption[T]) error
	Enqueue(ctx context.Context, name string, msg T) error
}

type Config struct {
	Logger     worker.StandardLogger
	MaxWorkers uint
	MaxRetries uint
	MaxTimeout time.Duration
}
type Option func(c *Config) error

func WithLogger(l worker.StandardLogger) Option {
	return func(c *Config) error {
		if l == nil {
			return errors.New("job queue logger cannot be nil")
		}
		c.Logger = l
		return nil
	}
}

func WithMaxWorkers(maxWorkers uint) Option {
	return func(c *Config) error {
		if maxWorkers < 1 {
			return errors.New("job queue max workers must be greater than zero")
		}
		c.MaxWorkers = maxWorkers
		return nil
	}
}

func WithMaxRetries(maxRetries uint) Option {
	return func(c *Config) error {
		c.MaxRetries = maxRetries
		return nil
	}
}

func WithMaxTimeout(maxTimeout time.Duration) Option {
	return func(c *Config) error {
		if maxTimeout == 0 {
			return errors.New("max timeout cannot be 0")
		}
		c.MaxTimeout = maxTimeout
		return nil
	}
}

type JobQueue[T any] struct {
	worker *worker.Worker[T]
	queue  *queue.Queue
	name   string

	// shutdown management
	mu          sync.Mutex
	stopping    bool
	startCtx    context.Context
	startCancel context.CancelFunc
	startWg     sync.WaitGroup
}

func New[T any](name string, db *sql.DB, ser serializer.Serializer[T], opts ...Option) (*JobQueue[T], error) {
	// set defaults
	c := &Config{
		Logger:     &worker.DiscardLogger{},
		MaxWorkers: 1,
		MaxRetries: 3,
		MaxTimeout: 5 * time.Second,
	}
	// apply overrides of defaults
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	// instantiate queue schema in the database, this should be fairly quick
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	if err := queue.Setup(ctx, db); err != nil {
		return nil, err
	}

	// instantiate queue
	q, err := queue.New(queue.NewOpts{
		DB:         db,
		MaxReceive: int(c.MaxRetries),
		Name:       name,
		Timeout:    c.MaxTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create queue: %w", err)
	}

	// instantiate worker which consumes from queue
	w := worker.New[T](q, ser, worker.WithLog(c.Logger), worker.WithLimit(int(c.MaxWorkers)))

	return &JobQueue[T]{
		queue:  q,
		worker: w,
		name:   name,
	}, nil
}

func (j *JobQueue[T]) Start(ctx context.Context) error {
	j.mu.Lock()
	if j.startCtx != nil {
		// Already started, this error is almost surly a developer error
		j.mu.Unlock()
		return fmt.Errorf("JobQueue[%s] already started", j.name)
	}
	j.startCtx, j.startCancel = context.WithCancel(ctx)
	j.startWg.Add(1)
	j.mu.Unlock()

	log.Infof("JobQueue[%s] starting", j.name)
	go func() {
		defer j.startWg.Done()
		j.worker.Start(j.startCtx)
		log.Infof("JobQueue[%s] worker stopped", j.name)
	}()
	return nil
}

func (j *JobQueue[T]) Register(name string, fn func(context.Context, T) error, opts ...worker.JobOption[T]) error {
	j.mu.Lock()
	if j.startCtx != nil {
		j.mu.Unlock()
		return fmt.Errorf("JobQueue[%s] already started, cannot register job on running job queue", j.name)
	}
	j.mu.Unlock()
	return j.worker.Register(name, fn, opts...)
}

func (j *JobQueue[T]) Enqueue(ctx context.Context, name string, msg T) error {
	j.mu.Lock()
	if j.startCtx == nil {
		j.mu.Unlock()
		return fmt.Errorf("JobQueue[%s] not started, must start before enqueuing a job", j.name)
	}
	if j.stopping {
		j.mu.Unlock()
		log.Debugf("JobQueue[%s] rejecting enqueue of %s - queue is stopping", j.name, name)
		return errors.New("job queue is stopping")
	}
	j.mu.Unlock()
	return j.worker.Enqueue(ctx, name, msg)
}

func (j *JobQueue[T]) Stop(ctx context.Context) error {
	j.mu.Lock()
	if j.startCtx == nil {
		j.mu.Unlock()
		return fmt.Errorf("JobQueue[%s] not started, must start before stopping job", j.name)
	}
	if j.stopping {
		j.mu.Unlock()
		log.Warnf("JobQueue[%s] already stopping, ignoring Stop call", j.name)
		return errors.New("job queue is already stopping")
	}
	j.stopping = true
	log.Infof("JobQueue[%s] stopping - no new tasks will be accepted", j.name)

	// Cancel the start context to signal worker to stop
	j.startCancel()
	j.mu.Unlock()

	log.Infof("JobQueue[%s] waiting for active tasks to complete", j.name)

	// Wait for the worker to finish processing all running tasks
	done := make(chan struct{})
	go func() {
		j.startWg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		log.Errorf("JobQueue[%s] stop timeout - some tasks may not have completed gracefully", j.name)
		return fmt.Errorf("stop timeout: %w", ctx.Err())
	case <-done:
		log.Infof("JobQueue[%s] stopped successfully - all tasks completed", j.name)
		return nil
	}
}

// WithOnFailure sets a callback to be invoked only when the job fails after max retries
// The JobQueue only supports a single OnFailure callback for a job, multiple OnFailure options must not be provided.
func WithOnFailure[T any](onFailure worker.OnFailureFn[T]) worker.JobOption[T] {
	return worker.WithOnFailure[T](onFailure)
}

func NewPermanentError(err error) error {
	return worker.Permanent(err)
}
