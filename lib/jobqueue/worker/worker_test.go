// Copyright (c) https://github.com/maragudk/goqite
// https://github.com/maragudk/goqite/blob/6d1bf3c0bcab5a683e0bc7a82a4c76ceac1bbe3f/LICENSE
//
// This source code is licensed under the MIT license found in the LICENSE file
// in the root directory of this source tree, or at:
// https://opensource.org/licenses/MIT

package worker_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	internalsql "github.com/storacha/piri/lib/jobqueue/internal/sql"
	internaltesting "github.com/storacha/piri/lib/jobqueue/internal/testing"
	"github.com/storacha/piri/lib/jobqueue/queue"
	"github.com/storacha/piri/lib/jobqueue/worker"
)

func TestRunner_Register(t *testing.T) {
	t.Run("can register a new job", func(t *testing.T) {
		r := worker.New[[]byte](nil, nil)
		require.NoError(t, r.Register("test", func(ctx context.Context, m []byte) error {
			return nil
		}))
	})

	t.Run("errors if the same job is registered twice", func(t *testing.T) {
		r := worker.New[[]byte](nil, nil)
		err := r.Register("test", func(ctx context.Context, m []byte) error {
			return nil
		})
		require.NoError(t, err)
		err = r.Register("test", func(ctx context.Context, m []byte) error { return nil })
		require.Error(t, err)
	})
}

func TestOnFailure(t *testing.T) {
	t.Run("calls OnFailure after max retries", func(t *testing.T) {
		q := internaltesting.NewQ(t, queue.NewOpts{
			MaxReceive: 3, // Max 3 attempts
			Timeout:    10 * time.Millisecond,
		})
		r := worker.New[[]byte](
			q,
			&PassThroughSerializer[[]byte]{},
			worker.WithLimit(10),
		)

		var onFailureCalled bool
		var capturedMsg []byte
		var capturedErr error

		ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
		defer cancel()

		// Register a job that always fails
		err := r.Register("failing-job",
			func(ctx context.Context, m []byte) error {
				return fmt.Errorf("job failed")
			},
			worker.WithOnFailure(func(ctx context.Context, msg []byte, err error) error {
				onFailureCalled = true
				capturedMsg = msg
				capturedErr = err
				return err
			}),
		)
		require.NoError(t, err)

		// Enqueue the job
		err = r.Enqueue(ctx, "failing-job", []byte("test-message"))
		require.NoError(t, err)

		// Start the worker
		r.Start(ctx)

		// Verify OnFailure was called
		require.True(t, onFailureCalled, "OnFailure should have been called")
		require.Equal(t, []byte("test-message"), capturedMsg)
		require.Error(t, capturedErr)
		require.Contains(t, capturedErr.Error(), "job failed")
	})

	t.Run("does not call OnFailure on success", func(t *testing.T) {
		q := internaltesting.NewQ(t, queue.NewOpts{
			MaxReceive: 3,
			Timeout:    10 * time.Millisecond,
		})
		r := worker.New[[]byte](
			q,
			&PassThroughSerializer[[]byte]{},
			worker.WithLimit(10),
		)

		var onFailureCalled bool

		ctx, cancel := context.WithCancel(t.Context())

		// Register a job that succeeds
		err := r.Register("success-job",
			func(ctx context.Context, m []byte) error {
				cancel()
				return nil
			},
			worker.WithOnFailure(func(ctx context.Context, msg []byte, err error) error {
				onFailureCalled = true
				return nil
			}),
		)
		require.NoError(t, err)

		// Enqueue the job
		err = r.Enqueue(ctx, "success-job", []byte("test"))
		require.NoError(t, err)

		// Start the worker
		r.Start(ctx)

		// Verify OnFailure was NOT called
		require.False(t, onFailureCalled, "OnFailure should not be called on success")
	})

	t.Run("does not call OnFailure before max retries", func(t *testing.T) {
		q := internaltesting.NewQ(t, queue.NewOpts{
			MaxReceive: 3, // Max 3 attempts
			Timeout:    10 * time.Millisecond,
		})
		r := worker.New[[]byte](
			q,
			&PassThroughSerializer[[]byte]{},
			worker.WithLimit(10),
		)

		var onFailureCalled bool
		var attempts int

		ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
		defer cancel()

		// Register a job that fails twice then succeeds
		err := r.Register("eventual-success",
			func(ctx context.Context, m []byte) error {
				attempts++
				if attempts < 3 {
					return fmt.Errorf("attempt %d failed", attempts)
				}
				cancel()
				return nil
			},
			worker.WithOnFailure(func(ctx context.Context, msg []byte, err error) error {
				onFailureCalled = true
				return nil
			}),
		)
		require.NoError(t, err)

		// Enqueue the job
		err = r.Enqueue(ctx, "eventual-success", []byte("test"))
		require.NoError(t, err)

		// Start the worker
		r.Start(ctx)

		// Verify OnFailure was NOT called
		require.False(t, onFailureCalled, "OnFailure should not be called if job eventually succeeds")
		require.Equal(t, 3, attempts, "Should have attempted 3 times")
	})
}

func TestDeadLetterQueue(t *testing.T) {
	t.Run("moves job to dead letter queue on PermanentError", func(t *testing.T) {
		db := internaltesting.NewInMemoryDB(t)
		q := internaltesting.NewQ(t, queue.NewOpts{
			DB:         db,
			MaxReceive: 3,
			Timeout:    10 * time.Millisecond,
		})
		r := worker.New[[]byte](
			q,
			&PassThroughSerializer[[]byte]{},
			worker.WithLimit(10),
		)

		ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
		defer cancel()

		// Register a job that returns a permanent error
		err := r.Register("permanent-error-job", func(ctx context.Context, m []byte) error {
			cancel()
			return worker.Permanent(fmt.Errorf("this is a permanent error"))
		})
		require.NoError(t, err)

		// Enqueue the job
		err = r.Enqueue(ctx, "permanent-error-job", []byte("test-message"))
		require.NoError(t, err)

		// Start the worker
		r.Start(ctx)

		// Verify the job is in the dead letter queue
		var count int
		err = db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM jobqueue_dead WHERE job_name = ? AND failure_reason = ?",
			"permanent-error-job", "permanent_error").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count, "Job should be in dead letter queue")

		// Verify the job is not in the main queue
		err = db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM jobqueue WHERE queue = ?", "test").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 0, count, "Job should not be in main queue")
	})

	t.Run("moves job to dead letter queue after max retries", func(t *testing.T) {
		db := internaltesting.NewInMemoryDB(t)
		q := internaltesting.NewQ(t, queue.NewOpts{
			DB:         db,
			MaxReceive: 3, // Max 3 attempts
			Timeout:    10 * time.Millisecond,
		})
		r := worker.New[[]byte](
			q,
			&PassThroughSerializer[[]byte]{},
			worker.WithLimit(10),
		)

		ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
		defer cancel()

		// Register a job that always fails
		err := r.Register("max-retries-job", func(ctx context.Context, m []byte) error {
			return fmt.Errorf("job failed")
		})
		require.NoError(t, err)

		// Enqueue the job
		err = r.Enqueue(ctx, "max-retries-job", []byte("test-message"))
		require.NoError(t, err)

		// Start the worker
		r.Start(ctx)

		// Verify the job is in the dead letter queue
		var count int
		err = db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM jobqueue_dead WHERE job_name = ? AND failure_reason = ?",
			"max-retries-job", "max_retries").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count, "Job should be in dead letter queue after max retries")

		// Verify the job is not in the main queue
		err = db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM jobqueue WHERE queue = ?", "test").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 0, count, "Job should not be in main queue")
	})

	t.Run("calls OnFailure before moving to dead letter queue", func(t *testing.T) {
		db := internaltesting.NewInMemoryDB(t)
		q := internaltesting.NewQ(t, queue.NewOpts{
			DB:         db,
			MaxReceive: 3,
			Timeout:    10 * time.Millisecond,
		})
		r := worker.New[[]byte](
			q,
			&PassThroughSerializer[[]byte]{},
			worker.WithLimit(10),
		)

		var onFailureCalled bool

		ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
		defer cancel()

		// Register a job that fails with OnFailure callback
		err := r.Register("failing-job-with-callback",
			func(ctx context.Context, m []byte) error {
				return fmt.Errorf("job failed")
			},
			worker.WithOnFailure(func(ctx context.Context, msg []byte, err error) error {
				onFailureCalled = true
				return nil
			}),
		)
		require.NoError(t, err)

		// Enqueue the job
		err = r.Enqueue(ctx, "failing-job-with-callback", []byte("test-message"))
		require.NoError(t, err)

		// Start the worker
		r.Start(ctx)

		// Verify OnFailure was called
		require.True(t, onFailureCalled, "OnFailure should have been called before moving to DLQ")

		// Verify the job is in the dead letter queue
		var count int
		err = db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM jobqueue_dead WHERE job_name = ?",
			"failing-job-with-callback").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count, "Job should be in dead letter queue after OnFailure")
	})
}

func TestRunner_Start(t *testing.T) {
	t.Run("can run a named job", func(t *testing.T) {
		_, r := newRunner(t)

		var ran bool
		ctx, cancel := context.WithCancel(t.Context())
		err := r.Register("test", func(ctx context.Context, m []byte) error {
			ran = true
			require.Equal(t, "yo", string(m))
			cancel()
			return nil
		})
		require.NoError(t, err)

		err = r.Enqueue(ctx, "test", []byte("yo"))
		require.NoError(t, err)

		r.Start(ctx)
		require.True(t, ran)
	})

	t.Run("doesn't run a different job", func(t *testing.T) {
		_, r := newRunner(t)

		var ranTest, ranDifferentTest bool
		ctx, cancel := context.WithCancel(t.Context())
		require.NoError(t, r.Register("test", func(ctx context.Context, m []byte) error {
			ranTest = true
			return nil
		}))
		require.NoError(t, r.Register("different-test", func(ctx context.Context, m []byte) error {
			ranDifferentTest = true
			cancel()
			return nil
		}))

		err := r.Enqueue(ctx, "different-test", []byte("yo"))
		require.NoError(t, err)

		r.Start(ctx)
		require.True(t, !ranTest)
		require.True(t, ranDifferentTest)
	})

	t.Run("panics if the job is not registered", func(t *testing.T) {
		_, r := newRunner(t)

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()

		err := r.Enqueue(ctx, "test", []byte("yo"))
		require.NoError(t, err)

		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("did not panic")
			}
			require.Equal(t, `job "test" not registered`, r)
		}()
		r.Start(ctx)
	})

	t.Run("does not panic if job panics", func(t *testing.T) {
		_, r := newRunner(t)

		ctx, cancel := context.WithCancel(t.Context())

		require.NoError(t, r.Register("test", func(ctx context.Context, m []byte) error {
			cancel()
			panic("test panic")
		}))

		err := r.Enqueue(ctx, "test", []byte("yo"))
		require.NoError(t, err)

		r.Start(ctx)
	})

	t.Run("extends a job's timeout if it takes longer than the default timeout", func(t *testing.T) {
		_, r := newRunner(t)

		var runCount int
		ctx, cancel := context.WithCancel(t.Context())
		require.NoError(t, r.Register("test", func(ctx context.Context, m []byte) error {
			runCount++
			// This is more than the default timeout, so it should extend
			time.Sleep(150 * time.Millisecond)
			cancel()
			return nil
		}))

		err := r.Enqueue(ctx, "test", []byte("yo"))
		require.NoError(t, err)

		r.Start(ctx)
		require.Equal(t, 1, runCount)
	})
}

func TestCreateTx(t *testing.T) {
	t.Run("can create a job inside a transaction", func(t *testing.T) {
		db := internaltesting.NewInMemoryDB(t)
		q := internaltesting.NewQ(t, queue.NewOpts{DB: db})
		r := worker.New[[]byte](q, &PassThroughSerializer[[]byte]{})

		var ran bool
		ctx, cancel := context.WithCancel(t.Context())
		require.NoError(t, r.Register("test", func(ctx context.Context, m []byte) error {
			ran = true
			require.Equal(t, "yo", string(m))
			cancel()
			return nil
		}))

		err := internalsql.InTx(db, func(tx *sql.Tx) error {
			return r.EnqueueTx(ctx, tx, "test", []byte("yo"))
		})
		require.NoError(t, err)

		r.Start(ctx)
		require.True(t, ran)
	})
}

func newRunner(t *testing.T) (*queue.Queue, *worker.Worker[[]byte]) {
	t.Helper()

	q := internaltesting.NewQ(t, queue.NewOpts{Timeout: 100 * time.Millisecond})
	r := worker.New[[]byte](
		q,
		&PassThroughSerializer[[]byte]{},
		worker.WithLimit(10),
		worker.WithExtend(100*time.Millisecond),
	)
	return q, r
}

type PassThroughSerializer[T any] struct{}

func (p PassThroughSerializer[T]) Serialize(val T) ([]byte, error) {
	b, ok := any(val).([]byte)
	if !ok {
		return nil, fmt.Errorf("PassThroughSerializer only supports []byte, got %T", val)
	}
	return b, nil
}

func (p PassThroughSerializer[T]) Deserialize(data []byte) (T, error) {
	var zero T
	// We cast []byte back to T, but T must be []byte or we return an error:
	if _, ok := any(zero).([]byte); !ok {
		return zero, fmt.Errorf("PassThroughSerializer only supports T = []byte")
	}
	return any(data).(T), nil
}
