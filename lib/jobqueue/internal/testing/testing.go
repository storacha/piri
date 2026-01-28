// Copyright (c) https://github.com/maragudk/goqite
// https://github.com/maragudk/goqite/blob/6d1bf3c0bcab5a683e0bc7a82a4c76ceac1bbe3f/LICENSE
//
// This source code is licensed under the MIT license found in the LICENSE file
// in the root directory of this source tree, or at:
// https://opensource.org/licenses/MIT

package testing

import (
	"database/sql"
	_ "embed"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/storacha/piri/lib/jobqueue/queue"
	"github.com/storacha/piri/pkg/database/sqlitedb"
)

//go:embed schema.sql
var schema string

//go:embed schema.postgres.sql
var schemaPostgres string

// NewInMemoryDB creates a new in-memory SQLite database for testing
// with the classic queue schema initialized.
func NewInMemoryDB(t testing.TB) *sql.DB {
	t.Helper()
	db, err := sqlitedb.NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	_, err = db.Exec(schema)
	if err != nil {
		t.Fatal(err)
	}

	return db
}

// NewDBForBackend creates a new database connection for the given backend.
// For SQLite, it creates an in-memory database.
// For PostgreSQL, it connects to the test container and sets up the schema.
func NewDBForBackend(t testing.TB, backend Backend) *sql.DB {
	t.Helper()
	switch backend {
	case BackendPostgres:
		db := NewPostgresDB(t)
		// Setup combined schema for PostgreSQL
		_, err := db.Exec(schemaPostgres)
		if err != nil {
			t.Fatalf("setup postgres schema: %v", err)
		}
		// Truncate tables to ensure clean state for each test
		_, err = db.Exec(`TRUNCATE TABLE job_dead, job_done, jobs, job_ns, queues, jobqueue_dead, jobqueue CASCADE`)
		if err != nil {
			t.Fatalf("truncate postgres tables: %v", err)
		}
		return db
	default:
		return NewInMemoryDB(t)
	}
}

// NewQ creates a new queue using an in-memory SQLite database for testing.
func NewQ(t testing.TB, opts queue.NewOpts) *queue.Queue {
	t.Helper()

	if opts.DB == nil {
		opts.DB = NewInMemoryDB(t)
	}

	if opts.Name == "" {
		opts.Name = "test"
	}

	q, err := queue.New(opts)
	require.NoError(t, err)
	return q
}

// NewQForBackend creates a new queue using the specified backend for testing.
func NewQForBackend(t testing.TB, opts queue.NewOpts, backend Backend) *queue.Queue {
	t.Helper()

	if opts.DB == nil {
		opts.DB = NewDBForBackend(t, backend)
	}

	// For PostgreSQL, clean up tables between tests
	if backend.IsPostgres() {
		_, err := opts.DB.Exec(`TRUNCATE TABLE jobqueue_dead, jobqueue CASCADE`)
		require.NoError(t, err)
	}

	if opts.Name == "" {
		opts.Name = "test"
	}

	// Set the dialect based on the backend
	opts.Dialect = backend.Dialect()

	q, err := queue.New(opts)
	require.NoError(t, err)
	return q
}

type Logger func(msg string, args ...any)

func (f Logger) Info(msg string, args ...any) {
	f(msg, args...)
}

func NewLogger(t *testing.T) Logger {
	t.Helper()

	return Logger(func(msg string, args ...any) {
		logArgs := []any{msg}
		for i := 0; i < len(args); i += 2 {
			logArgs = append(logArgs, fmt.Sprintf("%v=%v", args[i], args[i+1]))
		}
		t.Log(logArgs...)
	})
}
