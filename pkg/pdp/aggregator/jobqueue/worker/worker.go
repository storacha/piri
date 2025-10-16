// Copyright (c) https://github.com/maragudk/goqite
// https://github.com/maragudk/goqite/blob/6d1bf3c0bcab5a683e0bc7a82a4c76ceac1bbe3f/LICENSE
//
// This source code is licensed under the MIT license found in the LICENSE file
// in the root directory of this source tree, or at:
// https://opensource.org/licenses/MIT

// Package jobqueue provides a [Worker] which can run registered job [Func]s by name, when a message for it is received
// on the underlying queue.
//
// It provides:
//   - Limit on how many jobs can be run simultaneously
//   - Automatic message timeout extension while the job is running
//   - Graceful shutdown
package worker

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/queue"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/serializer"
)

// JobFn is the job function to run.
type JobFn[T any] = func(ctx context.Context, msg T) error

// OnFailureFn is the function that runs if the job never completes successfully after all retries.
type OnFailureFn[T any] = func(ctx context.Context, msg T, err error) error

// jobRegistration holds a job function and its optional OnFailure callback
type jobRegistration[T any] struct {
	fn        JobFn[T]
	onFailure OnFailureFn[T] // Called only when max retries exhausted
}

type Worker[T any] struct {
	queue         *queue.Queue
	jobs          map[string]*jobRegistration[T]
	pollInterval  time.Duration
	extend        time.Duration
	jobCount      int
	jobCountLimit int
	jobCountLock  sync.RWMutex
	log           StandardLogger
	serializer    serializer.Serializer[T]
}

type NewOpts struct {
	Loger         StandardLogger
	JobCountLimit int
	PollInterval  time.Duration
	Extend        time.Duration
}

func New[T any](q *queue.Queue, ser serializer.Serializer[T], options ...Option) *Worker[T] {
	// Default config
	cfg := &Config{
		Log:           &DiscardLogger{},
		JobCountLimit: runtime.GOMAXPROCS(0),
		PollInterval:  100 * time.Millisecond,
		Extend:        5 * time.Second,
	}

	// Apply all provided options to the config
	for _, opt := range options {
		opt(cfg)
	}

	// Construct the Worker using the final config
	jq := &Worker[T]{
		jobs: make(map[string]*jobRegistration[T]),

		queue:      q,
		serializer: ser,

		log:           cfg.Log,
		jobCountLimit: cfg.JobCountLimit,
		pollInterval:  cfg.PollInterval,
		extend:        cfg.Extend,
	}
	return jq
}

type message struct {
	Name    string
	Message []byte
}

// Start the Worker, blocking until the given context is cancelled.
// When the context is cancelled, waits for the jobs to finish.
func (r *Worker[T]) Start(ctx context.Context) {
	var names []string
	for k := range r.jobs {
		names = append(names, k)
	}
	sort.Strings(names)

	r.log.Infow("Starting", "jobs", names)

	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			r.log.Infow("Stopping")
			wg.Wait()
			r.log.Infow("Stopped")
			return
		default:
			r.receiveAndRun(ctx, &wg)
		}
	}
}

// JobOption configures a job registration
type JobOption[T any] func(*jobRegistration[T])

// WithOnFailure sets a callback to be invoked only when the job fails after max retries
// The Worker only supports a single OnFailure callback for a job, multiple OnFailure options must not be provided.
func WithOnFailure[T any](onFailure OnFailureFn[T]) JobOption[T] {
	return func(jr *jobRegistration[T]) {
		jr.onFailure = onFailure
	}
}

// Register must be called before `Start`
func (r *Worker[T]) Register(name string, fn JobFn[T], opts ...JobOption[T]) error {
	if _, ok := r.jobs[name]; ok {
		return fmt.Errorf(`job "%v" already registered`, name)
	}

	reg := &jobRegistration[T]{
		fn: fn,
	}

	for _, opt := range opts {
		opt(reg)
	}

	r.jobs[name] = reg
	return nil
}

func (r *Worker[T]) Enqueue(ctx context.Context, name string, msg T) error {
	r.log.Debugf("Enqueue -> %s: %v", name, msg)
	m, err := r.serializer.Serialize(msg)
	if err != nil {
		return fmt.Errorf("serializer error: %w", err)
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(message{Name: name, Message: m}); err != nil {
		return err
	}
	return r.queue.Send(ctx, queue.Message{Body: buf.Bytes()})
}

func (r *Worker[T]) EnqueueTx(ctx context.Context, tx *sql.Tx, name string, msg T) error {
	m, err := r.serializer.Serialize(msg)
	if err != nil {
		return fmt.Errorf("serializer error: %w", err)
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(message{Name: name, Message: m}); err != nil {
		return err
	}
	return r.queue.SendTx(ctx, tx, queue.Message{Body: buf.Bytes()})
}

func (r *Worker[T]) receiveAndRun(ctx context.Context, wg *sync.WaitGroup) {
	r.jobCountLock.RLock()
	if r.jobCount == r.jobCountLimit {
		r.jobCountLock.RUnlock()
		// This is to avoid a busy loop
		time.Sleep(r.pollInterval)
		return
	} else {
		r.jobCountLock.RUnlock()
	}

	m, err := r.queue.ReceiveAndWait(ctx, r.pollInterval)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return
		}
		r.log.Errorw("Error receiving job", "error", err)
		// Sleep a bit to not hammer the queue if there's an error with it
		time.Sleep(time.Second)
		return
	}

	if m == nil {
		return
	}

	var jm message
	if err := json.NewDecoder(bytes.NewReader(m.Body)).Decode(&jm); err != nil {
		r.log.Errorw("Error decoding job message body", "error", err)
		return
	}

	jobInput, err := r.serializer.Deserialize(jm.Message)
	if err != nil {
		r.log.Errorw("Error deserializing job message", "error", err)
		return
	}

	r.log.Debugw("Dequeue -> %s: %v", jm.Name, jobInput)
	jobReg, ok := r.jobs[jm.Name]
	if !ok {
		panic(fmt.Sprintf(`job "%v" not registered`, jm.Name))
	}

	r.jobCountLock.Lock()
	r.jobCount++
	r.jobCountLock.Unlock()

	wg.Add(1)
	go func() {
		defer wg.Done()

		defer func() {
			r.jobCountLock.Lock()
			r.jobCount--
			r.jobCountLock.Unlock()
		}()

		defer func() {
			if rec := recover(); rec != nil {
				r.log.Errorw("Recovered from panic in job", "error", rec)
			}
		}()

		jobCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		// Extend the job message while the job is running
		go func() {
			// Start by sleeping so we don't extend immediately
			time.Sleep(r.extend - r.extend/5)
			for {
				select {
				case <-jobCtx.Done():
					return
				default:
					r.log.Infow("Extending message timeout", "name", jm.Name)
					if err := r.queue.Extend(jobCtx, m.ID, r.extend); err != nil {
						r.log.Errorw("Error extending message timeout", "error", err)
					}
					time.Sleep(r.extend - r.extend/5)
				}
			}
		}()

		r.log.Infow("Running job", "name", jm.Name, "attempt", m.Received)
		before := time.Now()
		if err := jobReg.fn(jobCtx, jobInput); err != nil {
			var permanent *PermanentError
			if errors.As(err, &permanent) {
				r.log.Errorw("Failed to run job, PermanentError occurred", "error", permanent, "name", jm.Name)
				// Move to dead letter queue
				dlqCtx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				if dlqErr := r.queue.MoveToDeadLetter(dlqCtx, m.ID, jm.Name, "permanent_error", err.Error()); dlqErr != nil {
					r.log.Errorw("Error moving job to dead letter queue", "error", dlqErr, "original_error", err)
				} else {
					r.log.Infow("Moved job to dead letter queue", "name", jm.Name, "reason", "permanent_error")
				}
				return
			} else if m.Received == r.queue.MaxReceive() {
				r.log.Errorw("Failed to run job, max retries reached, will not retry",
					"name", jm.Name,
					"attempt", m.Received,
					"next_attempt", r.queue.Timeout(),
					"max_attempts", r.queue.MaxReceive(),
					"error", err,
				)
				// Invoke OnFailure callback if configured
				if jobReg.onFailure != nil {
					r.log.Infow("Invoking OnFailure callback", "name", jm.Name)
					if onFailErr := jobReg.onFailure(jobCtx, jobInput, err); onFailErr != nil {
						// this is a VERY critical error
						r.log.Errorw("Error invoking OnFailure callback", "name", jm.Name, "error", onFailErr)
					}
				}
				// Move to dead letter queue
				dlqCtx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				if dlqErr := r.queue.MoveToDeadLetter(dlqCtx, m.ID, jm.Name, "max_retries", err.Error()); dlqErr != nil {
					r.log.Errorw("Error moving job to dead letter queue", "error", dlqErr, "original_error", err)
				} else {
					r.log.Infow("Moved job to dead letter queue", "name", jm.Name, "reason", "max_retries")
				}
				return
			} else {
				r.log.Warnw("Error running job, retrying",
					"name", jm.Name,
					"attempt", m.Received,
					"max_attempts", r.queue.MaxReceive(),
					"error", err,
				)
				// Return early for retryable errors - message will be retried
				return
			}
		}
		duration := time.Since(before)
		r.log.Infow("Ran job", "name", jm.Name, "duration", duration, "attempt", m.Received)

		deleteCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		// TODO(forrest): we don't want to retry failures here if delete fails, this should be rare, but worth fixing.
		if err := r.queue.Delete(deleteCtx, m.ID); err != nil {
			r.log.Errorw("Error deleting job from queue, it will be retried", "error", err)
		}
	}()
}
