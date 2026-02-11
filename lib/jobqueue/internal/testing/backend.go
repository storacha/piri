package testing

import (
	"testing"

	"github.com/storacha/piri/lib/jobqueue/dialect"
)

// Backend represents a database backend for testing.
type Backend string

const (
	// BackendSQLite represents the SQLite backend.
	BackendSQLite Backend = "sqlite"
	// BackendPostgres represents the PostgreSQL backend.
	BackendPostgres Backend = "postgres"
)

// AllBackends returns all available backends for testing.
// PostgreSQL is excluded if not available (container not running or skipped).
func AllBackends() []Backend {
	backends := []Backend{BackendSQLite}
	if PostgresAvailable() {
		backends = append(backends, BackendPostgres)
	}
	return backends
}

// RunForAllBackends runs a test function for each available backend.
// This creates subtests named "sqlite" and "postgres".
func RunForAllBackends(t *testing.T, fn func(t *testing.T, backend Backend)) {
	for _, backend := range AllBackends() {
		backend := backend // capture range variable
		t.Run(string(backend), func(t *testing.T) {
			fn(t, backend)
		})
	}
}

// RunForAllBackendsB runs a benchmark function for each available backend.
func RunForAllBackendsB(b *testing.B, fn func(b *testing.B, backend Backend)) {
	for _, backend := range AllBackends() {
		backend := backend
		b.Run(string(backend), func(b *testing.B) {
			fn(b, backend)
		})
	}
}

// Dialect returns the SQL dialect for this backend.
func (b Backend) Dialect() dialect.Dialect {
	switch b {
	case BackendPostgres:
		return dialect.Postgres
	default:
		return dialect.SQLite
	}
}

// IsPostgres returns true if this is the PostgreSQL backend.
func (b Backend) IsPostgres() bool {
	return b == BackendPostgres
}

// IsSQLite returns true if this is the SQLite backend.
func (b Backend) IsSQLite() bool {
	return b == BackendSQLite
}
