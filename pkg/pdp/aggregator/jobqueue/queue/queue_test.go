// Copyright (c) https://github.com/maragudk/goqite
// https://github.com/maragudk/goqite/blob/6d1bf3c0bcab5a683e0bc7a82a4c76ceac1bbe3f/LICENSE
//
// This source code is licensed under the MIT license found in the LICENSE file
// in the root directory of this source tree, or at:
// https://opensource.org/licenses/MIT

package queue_test

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/storacha/piri/pkg/database/sqlitedb"
	testing2 "github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/internal/testing"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/queue"
)

//go:embed schema.sql
var schema string

func TestQueue(t *testing.T) {
	t.Run("can send and receive and delete a message", func(t *testing.T) {
		q := newQ(t, queue.NewOpts{Timeout: time.Millisecond})

		m, err := q.Receive(t.Context())
		require.NoError(t, err)
		require.Nil(t, m)

		m = &queue.Message{
			Body: []byte("yo"),
		}

		err = q.Send(t.Context(), *m)
		require.NoError(t, err)

		m, err = q.Receive(t.Context())
		require.NoError(t, err)
		require.NotNil(t, m)
		require.Equal(t, "yo", string(m.Body))

		err = q.Delete(t.Context(), m.ID)
		require.NoError(t, err)

		time.Sleep(time.Millisecond)

		m, err = q.Receive(t.Context())
		require.NoError(t, err)
		require.Nil(t, m)
	})
}

func TestQueue_New(t *testing.T) {
	t.Run("errors if db is nil", func(t *testing.T) {
		_, err := queue.New(queue.NewOpts{Name: "test"})
		require.Error(t, err)
	})

	t.Run("errors if name is empty", func(t *testing.T) {
		_, err := queue.New(queue.NewOpts{DB: &sql.DB{}})
		require.Error(t, err)
	})

	t.Run("errors if max receive is negative", func(t *testing.T) {
		_, err := queue.New(queue.NewOpts{DB: &sql.DB{}, Name: "test", MaxReceive: -1})
		require.Error(t, err)
	})

	t.Run("errors if timeout is negative", func(t *testing.T) {
		_, err := queue.New(queue.NewOpts{DB: &sql.DB{}, Name: "test", Timeout: -1})
		require.Error(t, err)
	})
}

func TestQueue_Send(t *testing.T) {
	t.Run("panics if delay is negative", func(t *testing.T) {
		q := newQ(t, queue.NewOpts{})

		var err error
		defer func() {
			require.NoError(t, err)
			r := recover()
			require.Equal(t, "delay cannot be negative", r)
		}()

		err = q.Send(t.Context(), queue.Message{Delay: -1})
	})
}

func TestQueue_Receive(t *testing.T) {
	t.Run("does not receive a delayed message immediately", func(t *testing.T) {
		q := newQ(t, queue.NewOpts{})

		m := &queue.Message{
			Body:  []byte("yo"),
			Delay: time.Second,
		}

		err := q.Send(t.Context(), *m)
		require.NoError(t, err)

		m, err = q.Receive(t.Context())
		require.NoError(t, err)
		require.Nil(t, m)

		time.Sleep(time.Second)

		m, err = q.Receive(t.Context())
		require.NoError(t, err)
		require.NotNil(t, m)
		require.Equal(t, "yo", string(m.Body))
	})

	t.Run("does not receive a message twice in a row", func(t *testing.T) {
		q := newQ(t, queue.NewOpts{})

		m := &queue.Message{
			Body: []byte("yo"),
		}

		err := q.Send(t.Context(), *m)
		require.NoError(t, err)

		m, err = q.Receive(t.Context())
		require.NoError(t, err)
		require.NotNil(t, m)
		require.Equal(t, "yo", string(m.Body))

		m, err = q.Receive(t.Context())
		require.NoError(t, err)
		require.Nil(t, m)
	})

	t.Run("does receive a message up to two times if set and timeout has passed", func(t *testing.T) {
		q := newQ(t, queue.NewOpts{Timeout: time.Millisecond, MaxReceive: 2})

		m := &queue.Message{
			Body: []byte("yo"),
		}

		err := q.Send(t.Context(), *m)
		require.NoError(t, err)

		m, err = q.Receive(t.Context())
		require.NoError(t, err)
		require.NotNil(t, m)
		require.Equal(t, "yo", string(m.Body))

		time.Sleep(time.Millisecond)

		m, err = q.Receive(t.Context())
		require.NoError(t, err)
		require.NotNil(t, m)
		require.Equal(t, "yo", string(m.Body))

		time.Sleep(time.Millisecond)

		m, err = q.Receive(t.Context())
		require.NoError(t, err)
		require.Nil(t, m)
	})

	t.Run("does not receive a message from a different queue", func(t *testing.T) {
		q1 := newQ(t, queue.NewOpts{})
		q2 := newQ(t, queue.NewOpts{Name: "q2"})

		err := q1.Send(t.Context(), queue.Message{Body: []byte("yo")})
		require.NoError(t, err)

		m, err := q2.Receive(t.Context())
		require.NoError(t, err)
		require.Nil(t, m)
	})
}

func TestQueue_SendAndGetID(t *testing.T) {
	t.Run("returns the message ID", func(t *testing.T) {
		q := newQ(t, queue.NewOpts{})

		m := queue.Message{
			Body: []byte("yo"),
		}

		id, err := q.SendAndGetID(t.Context(), m)
		require.NoError(t, err)
		require.Equal(t, 34, len(id))

		err = q.Delete(t.Context(), id)
		require.NoError(t, err)
	})
}

func TestQueue_Extend(t *testing.T) {
	t.Run("does not receive a message that has had the timeout extended", func(t *testing.T) {
		q := newQ(t, queue.NewOpts{Timeout: time.Millisecond})

		m := &queue.Message{
			Body: []byte("yo"),
		}

		err := q.Send(t.Context(), *m)
		require.NoError(t, err)

		m, err = q.Receive(t.Context())
		require.NoError(t, err)
		require.NotNil(t, m)

		err = q.Extend(t.Context(), m.ID, time.Second)
		require.NoError(t, err)

		time.Sleep(time.Millisecond)

		m, err = q.Receive(t.Context())
		require.NoError(t, err)
		require.Nil(t, m)
	})

	t.Run("panics if delay is negative", func(t *testing.T) {
		q := newQ(t, queue.NewOpts{})

		var err error
		defer func() {
			require.NoError(t, err)
			r := recover()
			require.Equal(t, "delay cannot be negative", r)
		}()

		m := &queue.Message{
			Body: []byte("yo"),
		}

		err = q.Send(t.Context(), *m)
		require.NoError(t, err)

		m, err = q.Receive(t.Context())
		require.NoError(t, err)
		require.NotNil(t, m)

		err = q.Extend(t.Context(), m.ID, -1)
	})
}

func TestQueue_ReceiveAndWait(t *testing.T) {
	t.Run("waits for a message until the context is cancelled", func(t *testing.T) {
		q := newQ(t, queue.NewOpts{Timeout: time.Millisecond})

		ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
		defer cancel()

		m, err := q.ReceiveAndWait(ctx, time.Millisecond)
		require.Error(t, context.DeadlineExceeded, err)
		require.Nil(t, m)
	})

	t.Run("gets a message immediately if there is one", func(t *testing.T) {
		q := newQ(t, queue.NewOpts{Timeout: time.Millisecond})

		err := q.Send(t.Context(), queue.Message{Body: []byte("yo")})
		require.NoError(t, err)

		m, err := q.ReceiveAndWait(t.Context(), time.Millisecond)
		require.NoError(t, err)
		require.NotNil(t, m)
		require.Equal(t, "yo", string(m.Body))
	})
}

func TestSetup(t *testing.T) {
	t.Run("creates the database table", func(t *testing.T) {
		db, err := sqlitedb.NewMemory()
		if err != nil {
			t.Fatal(err)
		}
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)

		_, err = db.Exec(`select * from jobqueue`)
		require.Error(t, err)
		err = queue.Setup(t.Context(), db)
		require.NoError(t, err)
		_, err = db.Exec(`select * from jobqueue`)
		require.NoError(t, err)
	})
}

func TestQueue_MoveToDeadLetter(t *testing.T) {
	t.Run("moves a message to the dead letter queue", func(t *testing.T) {
		db := testing2.NewInMemoryDB(t)
		q, err := queue.New(queue.NewOpts{DB: db, Name: "test"})
		require.NoError(t, err)

		// Send a message
		m := queue.Message{Body: []byte("test message")}
		id, err := q.SendAndGetID(t.Context(), m)
		require.NoError(t, err)

		// Receive it to get the full message
		received, err := q.Receive(t.Context())
		require.NoError(t, err)
		require.NotNil(t, received)

		// Move to dead letter queue
		err = q.MoveToDeadLetter(t.Context(), received.ID, "test-job", "permanent_error", "test error")
		require.NoError(t, err)

		// Verify it's no longer in the main queue
		m2, err := q.Receive(t.Context())
		require.NoError(t, err)
		require.Nil(t, m2)

		// Verify it's in the dead letter queue
		var count int
		err = db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM jobqueue_dead WHERE id = ?", id).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		// Verify the metadata
		var jobName, failureReason, errorMessage string
		err = db.QueryRowContext(t.Context(),
			"SELECT job_name, failure_reason, error_message FROM jobqueue_dead WHERE id = ?",
			id).Scan(&jobName, &failureReason, &errorMessage)
		require.NoError(t, err)
		require.Equal(t, "test-job", jobName)
		require.Equal(t, "permanent_error", failureReason)
		require.Equal(t, "test error", errorMessage)
	})

	t.Run("errors if message does not exist", func(t *testing.T) {
		q := newQ(t, queue.NewOpts{})

		err := q.MoveToDeadLetter(t.Context(), "non-existent-id", "test-job", "permanent_error", "test error")
		require.Error(t, err)
		require.Contains(t, err.Error(), "not found")
	})

	t.Run("handles max_retries failure reason", func(t *testing.T) {
		db := testing2.NewInMemoryDB(t)
		q, err := queue.New(queue.NewOpts{DB: db, Name: "test"})
		require.NoError(t, err)

		// Send and receive a message
		m := queue.Message{Body: []byte("test message")}
		id, err := q.SendAndGetID(t.Context(), m)
		require.NoError(t, err)

		received, err := q.Receive(t.Context())
		require.NoError(t, err)
		require.NotNil(t, received)

		// Move to dead letter queue with max_retries reason
		err = q.MoveToDeadLetter(t.Context(), received.ID, "test-job", "max_retries", "failed after 3 attempts")
		require.NoError(t, err)

		// Verify the failure reason
		var failureReason string
		err = db.QueryRowContext(t.Context(),
			"SELECT failure_reason FROM jobqueue_dead WHERE id = ?",
			id).Scan(&failureReason)
		require.NoError(t, err)
		require.Equal(t, "max_retries", failureReason)
	})
}

func BenchmarkQueue(b *testing.B) {
	b.Run("send, receive, delete", func(b *testing.B) {
		q := newQ(b, queue.NewOpts{})

		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				err := q.Send(b.Context(), queue.Message{
					Body: []byte("yo"),
				})
				require.NoError(b, err)

				m, err := q.Receive(b.Context())
				require.NoError(b, err)
				require.NotNil(b, m)

				err = q.Delete(b.Context(), m.ID)
				require.NoError(b, err)
			}
		})
	})

	b.Run("receive and delete message on a big table with multiple queues", func(b *testing.B) {
		indexes := []struct {
			query string
			skip  bool
		}{
			{"-- no index", false},
			{"create index goqite_created_idx on queue (created);", true},
			{"create index goqite_created_idx on queue (created);create index goqite_queue_idx on queue (queue);", true},
			{"create index goqite_queue_created_idx on queue (queue, created);", true},
			{"create index goqite_queue_timeout_idx on queue (queue, timeout);", true},
			{"create index goqite_queue_created_timeout_idx on queue (queue, created, timeout);", true},
			{"create index goqite_queue_timeout_created_idx on queue (queue, timeout, created);", true},
		}

		for _, index := range indexes {
			b.Run(index.query, func(b *testing.B) {
				if index.skip {
					b.SkipNow()
				}

				db := newDB(b, "bench.db")
				_, err := db.Exec(index.query)
				require.NoError(b, err)

				var queues []*queue.Queue
				for i := 0; i < 10; i++ {
					queues = append(queues, newQ(b, queue.NewOpts{Name: fmt.Sprintf("q%v", i)}))
				}

				for i := 0; i < 100_000; i++ {
					q := queues[rand.Intn(len(queues))]
					err := q.Send(b.Context(), queue.Message{
						Body: []byte("yo"),
					})
					require.NoError(b, err)
				}

				b.ResetTimer()

				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						q := queues[rand.Intn(len(queues))]

						m, err := q.Receive(b.Context())
						require.NoError(b, err)

						err = q.Delete(b.Context(), m.ID)
						require.NoError(b, err)
					}
				})
			})
		}
	})
}

func newDB(t testing.TB, path string) *sql.DB {
	t.Helper()

	// Check if file exists already
	exists := false
	if _, err := os.Stat(path); err == nil {
		exists = true
	}

	if path != ":memory:" && !exists {
		t.Cleanup(func() {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		})
	}

	var db *sql.DB
	var err error
	if path == ":memory:" {
		db, err = sqlitedb.NewMemory()
	} else {
		db, err = sqlitedb.New(path)

	}
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if !exists {
		_, err = db.Exec(schema)
		if err != nil {
			t.Fatal(err)
		}
	}

	return db
}

func newQ(t testing.TB, opts queue.NewOpts) *queue.Queue {
	t.Helper()

	opts.DB = testing2.NewInMemoryDB(t)

	if opts.Name == "" {
		opts.Name = "test"
	}

	q, err := queue.New(opts)
	require.NoError(t, err)
	return q
}
