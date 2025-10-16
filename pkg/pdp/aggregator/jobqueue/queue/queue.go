// Copyright (c) https://github.com/maragudk/goqite
// https://github.com/maragudk/goqite/blob/6d1bf3c0bcab5a683e0bc7a82a4c76ceac1bbe3f/LICENSE
//
// This source code is licensed under the MIT license found in the LICENSE file
// in the root directory of this source tree, or at:
// https://opensource.org/licenses/MIT

package queue

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"time"

	internalsql "github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/internal/sql"
)

//go:embed schema.sql
var schema string

// rfc3339Milli is like time.RFC3339Nano, but with millisecond precision, and fractional seconds do not have trailing
// zeros removed.
const rfc3339Milli = "2006-01-02T15:04:05.000Z07:00"

type NewOpts struct {
	DB         *sql.DB
	MaxReceive int // Max receive count for messages before they cannot be received anymore.
	Name       string
	Timeout    time.Duration // Default timeout for messages before they can be re-received.
}

// New Queue with the given options.
// Defaults if not given:
// - Logs are discarded.
// - Max receive count is 3.
// - Timeout is five seconds.
func New(opts NewOpts) (*Queue, error) {
	if opts.DB == nil {
		return nil, errors.New("db is required")
	}

	// TODO(forrest): check if a queue with name already exists and fail if the case.
	if opts.Name == "" {
		return nil, errors.New("queue name is required")
	}

	if opts.MaxReceive < 0 {
		return nil, errors.New("max receive cannot negative")
	}

	if opts.MaxReceive == 0 {
		opts.MaxReceive = 3
	}

	if opts.Timeout < 0 {
		return nil, errors.New("timeout cannot be negative")
	}

	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Second
	}

	return &Queue{
		db:         opts.DB,
		name:       opts.Name,
		maxReceive: opts.MaxReceive,
		timeout:    opts.Timeout,
	}, nil
}

type Queue struct {
	db         *sql.DB
	maxReceive int
	name       string
	timeout    time.Duration
}

type ID string

type Message struct {
	ID       ID
	Delay    time.Duration
	Received int
	Body     []byte
}

func (q *Queue) MaxReceive() int {
	return q.maxReceive
}

func (q *Queue) Timeout() time.Duration {
	return q.timeout
}

// Send a Message to the queue with an optional delay.
func (q *Queue) Send(ctx context.Context, m Message) error {
	return internalsql.InTx(q.db, func(tx *sql.Tx) error {
		return q.SendTx(ctx, tx, m)
	})
}

// SendTx is like Send, but within an existing transaction.
func (q *Queue) SendTx(ctx context.Context, tx *sql.Tx, m Message) error {
	_, err := q.sendAndGetIDTx(ctx, tx, m)
	return err
}

// SendAndGetID is like Send, but also returns the message ID, which can be used
// to interact with the message without receiving it first.
func (q *Queue) SendAndGetID(ctx context.Context, m Message) (ID, error) {
	var id ID
	err := internalsql.InTx(q.db, func(tx *sql.Tx) error {
		var err error
		id, err = q.sendAndGetIDTx(ctx, tx, m)
		return err
	})
	return id, err
}

// sendAndGetIDTx is like SendAndGetID, but within an existing transaction.
func (q *Queue) sendAndGetIDTx(ctx context.Context, tx *sql.Tx, m Message) (ID, error) {
	if m.Delay < 0 {
		panic("delay cannot be negative")
	}

	timeout := time.Now().Add(m.Delay).Format(rfc3339Milli)

	var id ID
	query := `insert into jobqueue (queue, body, timeout) values (?, ?, ?) returning id`
	if err := tx.QueryRowContext(ctx, query, q.name, m.Body, timeout).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

// Receive a Message from the queue, or nil if there is none.
func (q *Queue) Receive(ctx context.Context) (*Message, error) {
	var m *Message
	err := internalsql.InTx(q.db, func(tx *sql.Tx) error {
		var err error
		m, err = q.receiveTx(ctx, tx)
		return err
	})
	return m, err
}

// receiveTx is like Receive, but within an existing transaction.
func (q *Queue) receiveTx(ctx context.Context, tx *sql.Tx) (*Message, error) {
	now := time.Now()
	nowFormatted := now.Format(rfc3339Milli)
	timeoutFormatted := now.Add(q.timeout).Format(rfc3339Milli)

	query := `
		update jobqueue
		set
			timeout = ?,
			received = received + 1
		where id = (
			select id from jobqueue
			where
				queue = ? and
				? >= timeout and
				received < ?
			order by created
			limit 1
		)
		returning id, body, received`

	var m Message
	if err := tx.QueryRowContext(ctx, query, timeoutFormatted, q.name, nowFormatted, q.maxReceive).Scan(&m.ID, &m.Body, &m.Received); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// ReceiveAndWait for a Message from the queue, polling at the given interval, until the context is cancelled.
// If the context is cancelled, the error will be non-nil. See [context.Context.Err].
func (q *Queue) ReceiveAndWait(ctx context.Context, interval time.Duration) (*Message, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			m, err := q.Receive(ctx)
			if err != nil {
				return nil, err
			}
			if m != nil {
				return m, nil
			}
		}
	}
}

// Extend a Message timeout by the given delay from now.
func (q *Queue) Extend(ctx context.Context, id ID, delay time.Duration) error {
	return internalsql.InTx(q.db, func(tx *sql.Tx) error {
		return q.extendTx(ctx, tx, id, delay)
	})
}

// extendTx is like Extend, but within an existing transaction.
func (q *Queue) extendTx(ctx context.Context, tx *sql.Tx, id ID, delay time.Duration) error {
	if delay < 0 {
		panic("delay cannot be negative")
	}

	timeout := time.Now().Add(delay).Format(rfc3339Milli)

	_, err := tx.ExecContext(ctx, `update jobqueue set timeout = ? where queue = ? and id = ?`, timeout, q.name, id)
	return err
}

// Delete a Message from the queue by id.
func (q *Queue) Delete(ctx context.Context, id ID) error {
	return internalsql.InTx(q.db, func(tx *sql.Tx) error {
		return q.deleteTx(ctx, tx, id)
	})
}

// deleteTx is like Delete, but within an existing transaction.
func (q *Queue) deleteTx(ctx context.Context, tx *sql.Tx, id ID) error {
	_, err := tx.ExecContext(ctx, `delete from jobqueue where queue = ? and id = ?`, q.name, id)
	return err
}

// MoveToDeadLetter moves a message from the main queue to the dead letter queue.
// This is used for jobs that fail permanently or exceed max retries.
func (q *Queue) MoveToDeadLetter(ctx context.Context, id ID, jobName, failureReason, errorMsg string) error {
	return internalsql.InTx(q.db, func(tx *sql.Tx) error {
		return q.moveToDeadLetterTx(ctx, tx, id, jobName, failureReason, errorMsg)
	})
}

// moveToDeadLetterTx is like MoveToDeadLetter, but within an existing transaction.
func (q *Queue) moveToDeadLetterTx(ctx context.Context, tx *sql.Tx, id ID, jobName, failureReason, errorMsg string) error {
	movedAt := time.Now().Format(rfc3339Milli)

	// First, copy the message to the dead letter queue
	insertQuery := `
		insert into jobqueue_dead (id, created, updated, queue, body, timeout, received, job_name, failure_reason, error_message, moved_at)
		select id, created, updated, queue, body, timeout, received, ?, ?, ?, ?
		from jobqueue
		where queue = ? and id = ?`

	result, err := tx.ExecContext(ctx, insertQuery, jobName, failureReason, errorMsg, movedAt, q.name, id)
	if err != nil {
		return fmt.Errorf("inserting into dead letter queue: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("message %s not found in queue %s", id, q.name)
	}

	// Then delete from the main queue
	if err := q.deleteTx(ctx, tx, id); err != nil {
		return fmt.Errorf("deleting from main queue: %w", err)
	}

	return nil
}

// Setup the queue in the database.
func Setup(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("setup queue schema: %w", err)
	}
	return nil
}
