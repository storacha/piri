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

	"github.com/storacha/piri/lib/jobqueue/logger"
	"github.com/storacha/piri/lib/jobqueue/queue"
	"github.com/storacha/piri/lib/jobqueue/serializer"
	"github.com/storacha/piri/lib/jobqueue/traceutil"
)

// JobFn is the job function to run.
type JobFn[T any] = func(ctx context.Context, msg T) error

// OnFailureFn is the function that runs if the job never completes successfully after all retries or returns a PermanentError.
type OnFailureFn[T any] = func(ctx context.Context, msg T, err error) error

// jobRegistration holds a job function and its optional OnFailure callback
type jobRegistration[T any] struct {
	fn        JobFn[T]
	onFailure OnFailureFn[T] // Called when max retries exhausted or PermanentError occurs
}

type Worker[T any] struct {
	queue         queue.Interface
	jobs          map[string]*jobRegistration[T]
	pollInterval  time.Duration
	extend        time.Duration
	jobCount      int
	jobCountLimit int
	jobCountLock  sync.RWMutex
	log           logger.StandardLogger
	serializer    serializer.Serializer[T]
}

// Config holds all parameters needed to initialize a Worker.
type Config struct {
	Log           logger.StandardLogger
	JobCountLimit int
	PollInterval  time.Duration
	Extend        time.Duration
}

// Option modifies a Config before creating the Worker.
type Option func(*Config)

func WithLog(l logger.StandardLogger) Option {
	return func(cfg *Config) {
		cfg.Log = l
	}
}

func WithLimit(limit int) Option {
	return func(cfg *Config) {
		cfg.JobCountLimit = limit
	}
}

func WithPollInterval(interval time.Duration) Option {
	return func(cfg *Config) {
		cfg.PollInterval = interval
	}
}

// WithExtend configures the frequency running jobs are polled for completion
func WithExtend(d time.Duration) Option {
	return func(cfg *Config) {
		cfg.Extend = d
	}
}

func New[T any](q queue.Interface, ser serializer.Serializer[T], options ...Option) *Worker[T] {
	// Default config
	cfg := &Config{
		Log:           &logger.DiscardLogger{},
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
	Name    string                        `json:"name"`
	Message []byte                        `json:"message"`
	Trace   *traceutil.SpanContextPayload `json:"trace,omitempty"`
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

// WithOnFailure sets a callback to be invoked when the job fails after max retries or returns a PermanentError
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

	traceInfo := traceutil.PayloadFromContext(ctx)

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(message{
		Name:    name,
		Message: m,
		Trace:   traceInfo,
	}); err != nil {
		return err
	}
	return r.queue.Send(ctx, queue.Message{Body: buf.Bytes()})
}

func (r *Worker[T]) EnqueueTx(ctx context.Context, tx *sql.Tx, name string, msg T) error {
	m, err := r.serializer.Serialize(msg)
	if err != nil {
		return fmt.Errorf("serializer error: %w", err)
	}

	traceInfo := traceutil.PayloadFromContext(ctx)

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(message{
		Name:    name,
		Message: m,
		Trace:   traceInfo,
	}); err != nil {
		return err
	}
	return r.queue.SendTx(ctx, tx, queue.Message{Body: buf.Bytes()})
}

func (r *Worker[T]) receiveAndRun(ctx context.Context, wg *sync.WaitGroup) {
	// Check if we've reached the worker limit
	r.jobCountLock.RLock()
	if r.jobCount == r.jobCountLimit {
		r.jobCountLock.RUnlock()
		time.Sleep(r.pollInterval) // Avoid busy loop
		return
	}
	r.jobCountLock.RUnlock()

	// Receive a message from the queue
	m, err := r.queue.ReceiveAndWait(ctx, r.pollInterval)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return
		}
		r.log.Errorw("Error receiving job", "error", err)
		time.Sleep(time.Second) // Avoid hammering the queue on errors
		return
	}

	if m == nil {
		return
	}

	// Decode and deserialize the message
	jm, jobInput, err := r.decodeMessage(m.Body)
	if err != nil {
		return // Error already logged
	}

	if sc, ok := traceutil.SpanContextFromPayload(jm.Trace); ok {
		ctx = traceutil.ContextWithLink(ctx, sc)
	}

	// Get the job registration
	jobReg, ok := r.jobs[jm.Name]
	if !ok {
		panic(fmt.Sprintf(`job "%v" not registered`, jm.Name))
	}

	// Increment job count and run the job asynchronously
	r.jobCountLock.Lock()
	r.jobCount++
	r.jobCountLock.Unlock()

	wg.Add(1)
	go r.runJob(ctx, wg, m, jm, jobInput, jobReg)
}

// decodeMessage decodes and deserializes a message body
func (r *Worker[T]) decodeMessage(body []byte) (message, T, error) {
	var jm message
	var zero T

	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&jm); err != nil {
		r.log.Errorw("Error decoding job message body", "error", err)
		return jm, zero, err
	}

	jobInput, err := r.serializer.Deserialize(jm.Message)
	if err != nil {
		r.log.Errorw("Error deserializing job message", "error", err)
		return jm, zero, err
	}

	r.log.Debugw("Dequeue -> %s: %v", jm.Name, jobInput)
	return jm, jobInput, nil
}

// runJob executes a job in a separate goroutine with timeout extension
func (r *Worker[T]) runJob(ctx context.Context, wg *sync.WaitGroup, m *queue.Message, jm message, jobInput T, jobReg *jobRegistration[T]) {
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

	// Start timeout extension goroutine
	go r.extendMessageTimeout(jobCtx, m.ID, jm.Name)

	// Execute the job
	r.log.Infow("Running job", "name", jm.Name, "attempt", m.Received)
	before := time.Now()
	if err := jobReg.fn(jobCtx, jobInput); err != nil {
		r.handleJobError(jobCtx, m, jm.Name, jobInput, jobReg, err)
		return
	}

	// Job succeeded
	duration := time.Since(before)
	r.log.Infow("Ran job", "name", jm.Name, "duration", duration, "attempt", m.Received)
	r.deleteMessage(m.ID)
}

// extendMessageTimeout periodically extends the message timeout while the job is running
func (r *Worker[T]) extendMessageTimeout(ctx context.Context, messageID queue.ID, jobName string) {
	time.Sleep(r.extend - r.extend/5) // Initial sleep
	for {
		select {
		case <-ctx.Done():
			return
		default:
			r.log.Infow("Extending message timeout", "name", jobName)
			if err := r.queue.Extend(ctx, messageID, r.extend); err != nil {
				r.log.Errorw("Error extending message timeout", "error", err)
			}
			time.Sleep(r.extend - r.extend/5)
		}
	}
}

// handleJobError handles different types of job errors: permanent, max retries, and retryable
func (r *Worker[T]) handleJobError(ctx context.Context, m *queue.Message, jobName string, jobInput T, jobReg *jobRegistration[T], err error) {
	var permanent *PermanentError
	if errors.As(err, &permanent) {
		r.handlePermanentError(ctx, m.ID, jobName, jobInput, jobReg, err)
		return
	}

	if m.Received == r.queue.MaxReceive() {
		r.handleMaxRetriesExceeded(ctx, m.ID, jobName, jobInput, jobReg, err, m.Received)
		return
	}

	// Retryable error
	r.log.Warnw("Error running job, retrying",
		"name", jobName,
		"attempt", m.Received,
		"max_attempts", r.queue.MaxReceive(),
		"error", err,
	)
}

// handlePermanentError handles errors that should not be retried
func (r *Worker[T]) handlePermanentError(ctx context.Context, messageID queue.ID, jobName string, jobInput T, jobReg *jobRegistration[T], err error) {
	r.log.Errorw("Failed to run job, PermanentError occurred", "error", err, "name", jobName)

	// Invoke OnFailure callback if configured
	if jobReg.onFailure != nil {
		r.invokeOnFailure(ctx, jobName, jobInput, jobReg.onFailure, err)
	}

	// Move to dead letter queue
	r.moveToDeadLetter(messageID, jobName, "permanent_error", err)
}

// handleMaxRetriesExceeded handles errors after all retries have been exhausted
func (r *Worker[T]) handleMaxRetriesExceeded(ctx context.Context, messageID queue.ID, jobName string, jobInput T, jobReg *jobRegistration[T], err error, attempt int) {
	r.log.Errorw("Failed to run job, max retries reached, will not retry",
		"name", jobName,
		"attempt", attempt,
		"next_attempt", r.queue.Timeout(),
		"max_attempts", r.queue.MaxReceive(),
		"error", err,
	)

	// Invoke OnFailure callback if configured
	if jobReg.onFailure != nil {
		r.invokeOnFailure(ctx, jobName, jobInput, jobReg.onFailure, err)
	}

	// Move to dead letter queue
	r.moveToDeadLetter(messageID, jobName, "max_retries", err)
}

// invokeOnFailure calls the OnFailure callback and logs any errors
func (r *Worker[T]) invokeOnFailure(ctx context.Context, jobName string, jobInput T, onFailure OnFailureFn[T], err error) {
	r.log.Infow("Invoking OnFailure callback", "name", jobName)
	if onFailErr := onFailure(ctx, jobInput, err); onFailErr != nil {
		r.log.Errorw("Error invoking OnFailure callback", "name", jobName, "error", onFailErr)
	}
}

// moveToDeadLetter moves a message to the dead letter queue
func (r *Worker[T]) moveToDeadLetter(messageID queue.ID, jobName string, reason string, err error) {
	// TODO PASS A CONTEXT FORREST
	dlqCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if dlqErr := r.queue.MoveToDeadLetter(dlqCtx, messageID, jobName, reason, err.Error()); dlqErr != nil {
		r.log.Errorw("Error moving job to dead letter queue", "error", dlqErr, "original_error", err)
	} else {
		r.log.Infow("Moved job to dead letter queue", "name", jobName, "reason", reason)
	}
}

// deleteMessage deletes a successfully processed message from the queue
func (r *Worker[T]) deleteMessage(messageID queue.ID) {
	deleteCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := r.queue.Delete(deleteCtx, messageID); err != nil {
		r.log.Errorw("Error deleting job from queue, it will be retried", "error", err)
	}
}
