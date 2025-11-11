package dedup

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	internalsql "github.com/storacha/piri/lib/jobqueue/internal/sql"
	"github.com/storacha/piri/lib/jobqueue/queue"
	"github.com/storacha/piri/lib/jobqueue/types"
)

//go:embed schema.sql
var schema string

type jobDoneStatus int

const (
	jobDoneStatusSuccess    jobDoneStatus = 1
	jobDoneStatusDeadLetter jobDoneStatus = 2
)

type messageEnvelope struct {
	Name    string
	Message []byte
}

type HashFunc func([]byte) []byte

type NewOpts struct {
	DB                *sql.DB
	Name              string
	MaxReceive        int
	Timeout           time.Duration
	DedupeEnabled     *bool
	BlockRepeatsOnDLQ *bool
	HashFunc          HashFunc
	Logger            types.StandardLogger
}

type Queue struct {
	db                *sql.DB
	name              string
	maxReceive        int
	timeout           time.Duration
	dedupeEnabled     bool
	blockRepeatsOnDLQ bool
	hash              HashFunc
	log               types.StandardLogger
}

func Setup(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("setup dedup queue schema: %w", err)
	}
	return nil
}

func New(opts NewOpts) (*Queue, error) {
	if opts.DB == nil {
		return nil, errors.New("db is required")
	}

	if opts.Name == "" {
		return nil, errors.New("queue name is required")
	}

	if opts.MaxReceive < 0 {
		return nil, errors.New("max receive cannot be negative")
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

	dedupeEnabled := true
	if opts.DedupeEnabled != nil {
		dedupeEnabled = *opts.DedupeEnabled
	}

	blockRepeatsOnDLQ := true
	if opts.BlockRepeatsOnDLQ != nil {
		blockRepeatsOnDLQ = *opts.BlockRepeatsOnDLQ
	}

	if opts.HashFunc == nil {
		opts.HashFunc = defaultHashFunc
	}
	if opts.Logger == nil {
		opts.Logger = &types.DiscardLogger{}
	}

	err := ensureQueueConfigured(opts.DB, opts.Name, dedupeEnabled)
	if err != nil {
		return nil, err
	}

	return &Queue{
		db:                opts.DB,
		name:              opts.Name,
		maxReceive:        opts.MaxReceive,
		timeout:           opts.Timeout,
		dedupeEnabled:     dedupeEnabled,
		blockRepeatsOnDLQ: blockRepeatsOnDLQ,
		hash:              opts.HashFunc,
		log:               opts.Logger,
	}, nil
}

func ensureQueueConfigured(db *sql.DB, name string, dedupeEnabled bool) error {
	_, err := db.Exec(`insert into queues(queue, dedupe_enabled) values(?, ?) on conflict(queue) do update set dedupe_enabled = excluded.dedupe_enabled`, name, boolToInt(dedupeEnabled))
	if err != nil {
		return fmt.Errorf("ensure queue configuration: %w", err)
	}
	return nil
}

func defaultHashFunc(payload []byte) []byte {
	sum := sha256.Sum256(payload)
	return sum[:]
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (q *Queue) MaxReceive() int {
	return q.maxReceive
}

func (q *Queue) Timeout() time.Duration {
	return q.timeout
}

func (q *Queue) Send(ctx context.Context, m queue.Message) error {
	return internalsql.InTx(q.db, func(tx *sql.Tx) error {
		_, err := q.sendAndGetIDTx(ctx, tx, m)
		return err
	})
}

func (q *Queue) SendTx(ctx context.Context, tx *sql.Tx, m queue.Message) error {
	_, err := q.sendAndGetIDTx(ctx, tx, m)
	return err
}

func (q *Queue) SendAndGetID(ctx context.Context, m queue.Message) (queue.ID, error) {
	var id queue.ID
	err := internalsql.InTx(q.db, func(tx *sql.Tx) error {
		var err error
		id, err = q.sendAndGetIDTx(ctx, tx, m)
		return err
	})
	return id, err
}

func (q *Queue) sendAndGetIDTx(ctx context.Context, tx *sql.Tx, m queue.Message) (queue.ID, error) {
	if m.Delay < 0 {
		panic("delay cannot be negative")
	}

	env, err := decodeEnvelope(m.Body)
	if err != nil {
		return "", err
	}

	nsID, err := q.ensureNamespace(ctx, tx, env.Name)
	if err != nil {
		return "", err
	}

	key := q.hash(env.Message)

	if q.dedupeEnabled {
		done, err := q.isJobDone(ctx, tx, nsID, key)
		if err != nil {
			return "", err
		}
		if done {
			q.log.Infow("skipping duplicate job", "name", env.Name)
			return "", nil
		}
	}

	available := time.Now().Add(m.Delay).Unix()

	var id int64
	insertQuery := `
		insert into jobs(ns_id, key, body, avail_s)
		values (?, ?, ?, ?)
		on conflict(ns_id, key) do nothing
		returning id`

	err = tx.QueryRowContext(ctx, insertQuery, nsID, key, m.Body, available).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			q.log.Infow("skipping duplicate job", "name", env.Name)
			return "", nil
		}
		return "", fmt.Errorf("insert job: %w", err)
	}

	return queue.ID(strconv.FormatInt(id, 10)), nil
}

func decodeEnvelope(body []byte) (*messageEnvelope, error) {
	var env messageEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decode message envelope: %w", err)
	}
	if env.Name == "" {
		return nil, errors.New("message envelope missing name")
	}
	return &env, nil
}

func (q *Queue) ensureNamespace(ctx context.Context, tx *sql.Tx, name string) (int64, error) {
	query := `insert into job_ns(queue, name) values(?, ?) on conflict(queue, name) do nothing`
	if _, err := tx.ExecContext(ctx, query, q.name, name); err != nil {
		return 0, fmt.Errorf("ensure namespace insert: %w", err)
	}

	var id int64
	selectQuery := `select id from job_ns where queue = ? and name = ?`
	if err := tx.QueryRowContext(ctx, selectQuery, q.name, name).Scan(&id); err != nil {
		return 0, fmt.Errorf("ensure namespace select: %w", err)
	}
	return id, nil
}

func (q *Queue) isJobDone(ctx context.Context, tx *sql.Tx, nsID int64, key []byte) (bool, error) {
	var status int
	err := tx.QueryRowContext(ctx, `select status from job_done where ns_id = ? and key = ?`, nsID, key).Scan(&status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("check job done: %w", err)
	}
	return true, nil
}

func (q *Queue) Receive(ctx context.Context) (*queue.Message, error) {
	var m *queue.Message
	err := internalsql.InTx(q.db, func(tx *sql.Tx) error {
		var err error
		m, err = q.receiveTx(ctx, tx)
		return err
	})
	return m, err
}

func (q *Queue) receiveTx(ctx context.Context, tx *sql.Tx) (*queue.Message, error) {
	now := time.Now()
	nowSecs := now.Unix()
	newAvail := now.Add(q.timeout).Unix()

	query := `
		with next_job as (
			select j.id
			from jobs j
			join job_ns ns on ns.id = j.ns_id
			where
				ns.queue = ? and
				j.avail_s <= ? and
				j.attempts < ?
			order by j.created_s, j.id
			limit 1
		)
		update jobs
		set attempts = attempts + 1,
			avail_s = ?
		where id = (select id from next_job)
		returning id, body, attempts`

	var (
		id       int64
		body     []byte
		attempts int
	)

	err := tx.QueryRowContext(ctx, query, q.name, nowSecs, q.maxReceive, newAvail).Scan(&id, &body, &attempts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("receive job: %w", err)
	}

	return &queue.Message{
		ID:       queue.ID(strconv.FormatInt(id, 10)),
		Body:     body,
		Received: attempts,
	}, nil
}

func (q *Queue) ReceiveAndWait(ctx context.Context, interval time.Duration) (*queue.Message, error) {
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

func (q *Queue) Extend(ctx context.Context, id queue.ID, delay time.Duration) error {
	return internalsql.InTx(q.db, func(tx *sql.Tx) error {
		return q.extendTx(ctx, tx, id, delay)
	})
}

func (q *Queue) extendTx(ctx context.Context, tx *sql.Tx, id queue.ID, delay time.Duration) error {
	if delay < 0 {
		panic("delay cannot be negative")
	}

	jobID, err := parseJobID(id)
	if err != nil {
		return err
	}

	newAvail := time.Now().Add(delay).Unix()
	_, err = tx.ExecContext(ctx, `update jobs set avail_s = ? where id = ?`, newAvail, jobID)
	if err != nil {
		return fmt.Errorf("extend job: %w", err)
	}
	return nil
}

func (q *Queue) Delete(ctx context.Context, id queue.ID) error {
	return internalsql.InTx(q.db, func(tx *sql.Tx) error {
		return q.deleteTx(ctx, tx, id, jobDoneStatusSuccess)
	})
}

func (q *Queue) deleteTx(ctx context.Context, tx *sql.Tx, id queue.ID, status jobDoneStatus) error {
	jobID, err := parseJobID(id)
	if err != nil {
		return err
	}

	row, err := q.fetchJob(ctx, tx, jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	if _, err := tx.ExecContext(ctx, `delete from jobs where id = ?`, jobID); err != nil {
		return fmt.Errorf("delete job: %w", err)
	}

	if q.dedupeEnabled {
		if err := q.insertJobDone(ctx, tx, row.namespaceID, row.key, status); err != nil {
			return err
		}
	}

	return nil
}

func (q *Queue) MoveToDeadLetter(ctx context.Context, id queue.ID, jobName, failureReason, errorMsg string) error {
	return internalsql.InTx(q.db, func(tx *sql.Tx) error {
		return q.moveToDeadLetterTx(ctx, tx, id, jobName, failureReason, errorMsg)
	})
}

func (q *Queue) moveToDeadLetterTx(ctx context.Context, tx *sql.Tx, id queue.ID, jobName, failureReason, errorMsg string) error {
	jobID, err := parseJobID(id)
	if err != nil {
		return err
	}

	row, err := q.fetchJob(ctx, tx, jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	insertQuery := `
		insert into job_dead(id, ns_id, key, body, attempts, reason, error)
		values(?, ?, ?, ?, ?, ?, ?)`
	_, err = tx.ExecContext(ctx, insertQuery, row.id, row.namespaceID, row.key, row.body, row.attempts, failureReason, errorMsg)
	if err != nil {
		return fmt.Errorf("insert job_dead: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `delete from jobs where id = ?`, jobID); err != nil {
		return fmt.Errorf("delete job during dead-letter move: %w", err)
	}

	if q.dedupeEnabled && q.blockRepeatsOnDLQ {
		if err := q.insertJobDone(ctx, tx, row.namespaceID, row.key, jobDoneStatusDeadLetter); err != nil {
			return err
		}
	}

	return nil
}

type jobRow struct {
	id          int64
	namespaceID int64
	key         []byte
	body        []byte
	attempts    int
}

func (q *Queue) fetchJob(ctx context.Context, tx *sql.Tx, id int64) (*jobRow, error) {
	query := `select id, ns_id, key, body, attempts from jobs where id = ?`
	var row jobRow
	err := tx.QueryRowContext(ctx, query, id).Scan(&row.id, &row.namespaceID, &row.key, &row.body, &row.attempts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("fetch job: %w", err)
	}
	return &row, nil
}

func (q *Queue) insertJobDone(ctx context.Context, tx *sql.Tx, nsID int64, key []byte, status jobDoneStatus) error {
	_, err := tx.ExecContext(ctx, `insert or ignore into job_done(ns_id, key, status) values(?, ?, ?)`, nsID, key, int(status))
	if err != nil {
		return fmt.Errorf("insert job_done: %w", err)
	}
	return nil
}

func parseJobID(id queue.ID) (int64, error) {
	parsed, err := strconv.ParseInt(string(id), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid job id %q: %w", id, err)
	}
	return parsed, nil
}
