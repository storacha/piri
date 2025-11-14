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
