package database

import (
	"errors"
	"time"

	"github.com/glebarez/go-sqlite"
	"github.com/jackc/pgx/v5/pgconn"
	sqlite3 "modernc.org/sqlite/lib"
)

// PostgreSQL error codes
// See: https://www.postgresql.org/docs/current/errcodes-appendix.html
const (
	pgUniqueViolation     = "23505" // unique_violation
	pgSerializationFail   = "40001" // serialization_failure
	pgDeadlockDetected    = "40P01" // deadlock_detected
	pgLockNotAvailable    = "55P03" // lock_not_available
	pgForeignKeyViolation = "23503" // foreign_key_violation
	pgCheckViolation      = "23514" // check_violation
	pgNotNullViolation    = "23502" // not_null_violation
)

var (
	lockedCodes = map[int]struct{}{
		sqlite3.SQLITE_LOCKED:        {},
		sqlite3.SQLITE_BUSY:          {},
		sqlite3.SQLITE_BUSY_SNAPSHOT: {},
		sqlite3.SQLITE_BUSY_TIMEOUT:  {},
		sqlite3.SQLITE_BUSY_RECOVERY: {},
	}
	constraintCodes = map[int]struct{}{
		sqlite3.SQLITE_CONSTRAINT:            {},
		sqlite3.SQLITE_CONSTRAINT_CHECK:      {},
		sqlite3.SQLITE_CONSTRAINT_COMMITHOOK: {},
		sqlite3.SQLITE_CONSTRAINT_DATATYPE:   {},
		sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY: {},
		sqlite3.SQLITE_CONSTRAINT_FUNCTION:   {},
		sqlite3.SQLITE_CONSTRAINT_NOTNULL:    {},
		sqlite3.SQLITE_CONSTRAINT_PINNED:     {},
		sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY: {},
		sqlite3.SQLITE_CONSTRAINT_ROWID:      {},
		sqlite3.SQLITE_CONSTRAINT_TRIGGER:    {},
		sqlite3.SQLITE_CONSTRAINT_UNIQUE:     {},
		sqlite3.SQLITE_CONSTRAINT_VTAB:       {},
	}
)

func IsLockedError(err error) bool {
	// Check SQLite errors
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		_, ok := lockedCodes[sqliteErr.Code()]
		return ok
	}

	// Check PostgreSQL errors
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgSerializationFail, pgDeadlockDetected, pgLockNotAvailable:
			return true
		}
	}
	return false
}

func IsUniqueConstraintError(err error) bool {
	// Check SQLite errors
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		_, ok := constraintCodes[sqliteErr.Code()]
		return ok
	}

	// Check PostgreSQL errors
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgUniqueViolation
	}
	return false
}

// IsConstraintError returns true if the error is any constraint violation (unique, foreign key, check, etc.)
func IsConstraintError(err error) bool {
	// Check SQLite errors
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		_, ok := constraintCodes[sqliteErr.Code()]
		return ok
	}

	// Check PostgreSQL errors
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgUniqueViolation, pgForeignKeyViolation, pgCheckViolation, pgNotNullViolation:
			return true
		}
	}
	return false
}

type JournalMode string

const (
	// JournalModeDELETE journaling mode is the normal behavior.
	// In the DELETE mode, the rollback journal is deleted at the conclusion of each transaction.
	// Indeed, the delete operation is the action that causes the transaction to commit.
	// (See the document titled Atomic Commit In SQLite for additional detail.)
	JournalModeDELETE JournalMode = "DELETE"

	// JournalModeTRUNCATE journaling mode commits transactions by truncating the rollback journal to zero-length instead
	// of deleting it. On many systems, truncating a file is much faster than deleting the file since the containing
	// directory does not need to be changed.
	JournalModeTRUNCATE JournalMode = "TRUNCATE"

	// JournalModePERSIST journaling mode prevents the rollback journal from being deleted at the end of each transaction.
	// Instead, the header of the journal is overwritten with zeros.
	// This will prevent other database connections from rolling the journal back.
	// The PERSIST journaling mode is useful as an optimization on platforms where deleting or truncating a file is much
	// more expensive than overwriting the first block of a file with zeros.
	// See also: PRAGMA journal_size_limit and SQLITE_DEFAULT_JOURNAL_SIZE_LIMIT.
	JournalModePERSIST JournalMode = "PERSIST"

	// JournalModeMEMORY journaling mode stores the rollback journal in volatile RAM.
	// This saves disk I/O but at the expense of database safety and integrity.
	// If the application using SQLite crashes in the middle of a transaction when the MEMORY journaling mode is set,
	// then the database file will very likely go corrupt.
	JournalModeMEMORY JournalMode = "MEMORY"

	// JournalModeWAL journaling mode uses a write-ahead log instead of a rollback journal to implement transactions.
	// The WAL journaling mode is persistent; after being set it stays in effect across multiple database connections and
	// after closing and reopening the database.
	// A database in WAL journaling mode can only be accessed by SQLite version 3.7.0 (2010-07-21) or later.
	JournalModeWAL JournalMode = "WAL"

	// JournalModeOFF journaling mode disables the rollback journal completely.
	// No rollback journal is ever created and hence there is never a rollback journal to delete.
	// The OFF journaling mode disables the atomic commit and rollback capabilities of SQLite.
	// The ROLLBACK command no longer works; it behaves in an undefined way.
	// Applications must avoid using the ROLLBACK command when the journal mode is OFF.
	// If the application crashes in the middle of a transaction when the OFF journaling mode is set, then the database
	// file will very likely go corrupt. Without a journal, there is no way for a statement to unwind partially completed
	// operations following a constraint error. This might also leave the database in a corrupted state.
	// For example, if a duplicate entry causes a CREATE UNIQUE INDEX statement to fail half-way through, it will leave
	// behind a partially created, and hence corrupt, index.
	// Because OFF journaling mode allows the database file to be corrupted using ordinary SQL, it is disabled when
	// SQLITE_DBCONFIG_DEFENSIVE is enabled.
	JournalModeOFF JournalMode = "OFF"
)

type SyncMode int

func (s SyncMode) String() string {
	switch s {
	case SyncModeOFF:
		return "OFF"
	case SyncModeNORMAL:
		return "NORMAL"
	case SyncModeFULL:
		return "FULL"
	case SyncModeEXTRA:
		return "EXTRA"
	}
	panic("developer error")
}

const (
	// SyncModeOFF (0), SQLite continues without syncing as soon as it has handed data off to the operating system.
	// If the application running SQLite crashes, the data will be safe, but the database might become corrupted if
	// the operating system crashes or the computer loses power before that data has been written to the disk surface.
	// On the other hand, commits can be orders of magnitude faster with synchronous OFF.
	SyncModeOFF SyncMode = 0

	// SyncModeNORMAL (1), the SQLite database engine will still sync at the most critical moments, but less often
	// than in FULL mode. There is a very small (though non-zero) chance that a power failure at just the wrong time
	// could corrupt the database in journal_mode=DELETE on an older filesystem. WAL mode is safe from corruption with
	// synchronous=NORMAL, and probably DELETE mode is safe too on modern filesystems.
	// WAL mode is always consistent with synchronous=NORMAL, but WAL mode does lose durability.
	// A transaction committed in WAL mode with synchronous=NORMAL might roll back following a power loss or system crash.
	// Transactions are durable across application crashes regardless of the synchronous setting or journal mode.
	// The synchronous=NORMAL setting is a good choice for most applications running in WAL mode.
	SyncModeNORMAL SyncMode = 1

	// SyncModeFULL (2), the SQLite database engine will use the xSync method of the VFS to ensure that all content is
	// safely written to the disk surface prior to continuing.
	// This ensures that an operating system crash or power failure will not corrupt the database.
	// FULL synchronous is very safe, but it is also slower. FULL is the most commonly used synchronous setting when not in WAL mode.
	SyncModeFULL SyncMode = 2

	// SyncModeEXTRA synchronous is like FULL, but with the addition that the directory containing a rollback journal is synced
	// after that journal is unlinked to commit a transaction in DELETE mode. EXTRA provides additional durability if
	// the commit is followed closely by a power loss.
	SyncModeEXTRA SyncMode = 3
)

type Option func(*Config) error

func WithJournalMode(mode JournalMode) Option {
	return func(o *Config) error {
		o.JournalMode = mode
		return nil
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(o *Config) error {
		o.Timeout = timeout
		return nil
	}
}

func WithForeignKeyConstraintsEnable(enabled bool) Option {
	return func(o *Config) error {
		o.ForeignKeyConstraintsEnable = enabled
		return nil
	}
}

func WithSyncMode(mode SyncMode) Option {
	return func(o *Config) error {
		o.SyncMode = mode
		return nil
	}
}

type Config struct {
	JournalMode                 JournalMode
	Timeout                     time.Duration
	ForeignKeyConstraintsEnable bool
	SyncMode                    SyncMode
}
