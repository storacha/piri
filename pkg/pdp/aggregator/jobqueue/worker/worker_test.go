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

	internalsql "github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/internal/sql"
	internaltesting "github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/internal/testing"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/queue"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/worker"
)

func TestRunner_Register(t *testing.T) {
	t.Run("can register a new job", func(t *testing.T) {
		r := worker.New[[]byte](nil, nil)
		r.Register("test", func(ctx context.Context, m []byte) error {
			return nil
		})
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

func TestRunner_Start(t *testing.T) {
	t.Run("can run a named job", func(t *testing.T) {
		_, r := newRunner(t)

		var ran bool
		ctx, cancel := context.WithCancel(t.Context())
		r.Register("test", func(ctx context.Context, m []byte) error {
			ran = true
			require.Equal(t, "yo", string(m))
			cancel()
			return nil
		})

		err := r.Enqueue(ctx, "test", []byte("yo"))
		require.NoError(t, err)

		r.Start(ctx)
		require.True(t, ran)
	})

	t.Run("doesn't run a different job", func(t *testing.T) {
		_, r := newRunner(t)

		var ranTest, ranDifferentTest bool
		ctx, cancel := context.WithCancel(t.Context())
		r.Register("test", func(ctx context.Context, m []byte) error {
			ranTest = true
			return nil
		})
		r.Register("different-test", func(ctx context.Context, m []byte) error {
			ranDifferentTest = true
			cancel()
			return nil
		})

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

		r.Register("test", func(ctx context.Context, m []byte) error {
			cancel()
			panic("test panic")
		})

		err := r.Enqueue(ctx, "test", []byte("yo"))
		require.NoError(t, err)

		r.Start(ctx)
	})

	t.Run("extends a job's timeout if it takes longer than the default timeout", func(t *testing.T) {
		_, r := newRunner(t)

		var runCount int
		ctx, cancel := context.WithCancel(t.Context())
		r.Register("test", func(ctx context.Context, m []byte) error {
			runCount++
			// This is more than the default timeout, so it should extend
			time.Sleep(150 * time.Millisecond)
			cancel()
			return nil
		})

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
		r.Register("test", func(ctx context.Context, m []byte) error {
			ran = true
			require.Equal(t, "yo", string(m))
			cancel()
			return nil
		})

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
