// Package testing provides test utilities for the jobqueue package.
package testing

import (
	"context"
	"database/sql"
	"os"
	"runtime"
	"sync"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	pgContainer *postgres.PostgresContainer
	pgConnStr   string
	pgOnce      sync.Once
	pgErr       error
)

// SetupPostgresContainer initializes the PostgreSQL container.
// Call this from TestMain to set up the container once for all tests in a package.
func SetupPostgresContainer(ctx context.Context) error {
	pgOnce.Do(func() {
		// Skip on darwin in CI (same pattern as MinIO tests)
		if runtime.GOOS == "darwin" {
			pgErr = nil // Not an error, just skip
			return
		}

		// Allow skipping via environment variable
		if os.Getenv("PIRI_SKIP_POSTGRES_TESTS") == "1" {
			return
		}

		pgContainer, pgErr = postgres.Run(ctx,
			"postgres:16-alpine",
			postgres.WithDatabase("testdb"),
			postgres.WithUsername("test"),
			postgres.WithPassword("test"),
			// wait for two occurrences of this log since its logged twice at startup:
			// 1. After initial database initialization
			// 2. After PostgreSQL restarts and is truly ready
			// This is a quirk with Postgres containers, only after the second log line is the database ready.
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2)),
		)
		if pgErr != nil {
			return
		}
		pgConnStr, pgErr = pgContainer.ConnectionString(ctx, "sslmode=disable")
	})
	return pgErr
}

// TeardownPostgresContainer terminates the PostgreSQL container.
// Call this from TestMain after m.Run() completes.
func TeardownPostgresContainer(ctx context.Context) {
	if pgContainer != nil {
		_ = pgContainer.Terminate(ctx)
	}
}

// PostgresAvailable returns true if PostgreSQL container is running and available.
func PostgresAvailable() bool {
	return pgConnStr != "" && pgErr == nil
}

// NewPostgresDB creates a new PostgreSQL connection for testing.
// It skips the test if PostgreSQL is not available.
func NewPostgresDB(t testing.TB) *sql.DB {
	t.Helper()
	if !PostgresAvailable() {
		t.Skip("PostgreSQL not available")
	}

	db, err := sql.Open("pgx", pgConnStr)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return db
}
