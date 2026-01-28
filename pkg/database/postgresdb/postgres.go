package postgresdb

import (
	"database/sql"
	"fmt"
	"net/url"
	"time"

	logging "github.com/ipfs/go-log/v2"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var log = logging.Logger("database")

const (
	// DefaultMaxOpenConns is the default maximum number of open connections.
	// PostgreSQL can handle many more connections than SQLite.
	DefaultMaxOpenConns = 25
	// DefaultMaxIdleConns is the default maximum number of idle connections.
	DefaultMaxIdleConns = 5
	// DefaultConnMaxLifetime is the default maximum connection lifetime.
	DefaultConnMaxLifetime = 5 * time.Minute
)

// Options configures a PostgreSQL connection.
type Options struct {
	// MaxOpenConns is the maximum number of open connections to the database.
	MaxOpenConns int
	// MaxIdleConns is the maximum number of idle connections in the pool.
	MaxIdleConns int
	// ConnMaxLifetime is the maximum amount of time a connection may be reused.
	ConnMaxLifetime time.Duration
}

// Option is a functional option for configuring PostgreSQL connections.
type Option func(*Options)

// WithMaxOpenConns sets the maximum number of open connections.
func WithMaxOpenConns(n int) Option {
	return func(o *Options) {
		o.MaxOpenConns = n
	}
}

// WithMaxIdleConns sets the maximum number of idle connections.
func WithMaxIdleConns(n int) Option {
	return func(o *Options) {
		o.MaxIdleConns = n
	}
}

// WithConnMaxLifetime sets the maximum connection lifetime.
func WithConnMaxLifetime(d time.Duration) Option {
	return func(o *Options) {
		o.ConnMaxLifetime = d
	}
}

// New creates a new PostgreSQL database connection.
// The connURL should be a PostgreSQL connection string in the format:
// postgres://user:password@host:port/dbname?sslmode=disable
// If schema is provided, a separate PostgreSQL schema will be created and used.
func New(connURL string, schema string, opts ...Option) (*sql.DB, error) {
	cfg := &Options{
		MaxOpenConns:    DefaultMaxOpenConns,
		MaxIdleConns:    DefaultMaxIdleConns,
		ConnMaxLifetime: DefaultConnMaxLifetime,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	// If schema is specified, append search_path to connection string
	dsn := connURL
	if schema != "" {
		u, err := url.Parse(connURL)
		if err != nil {
			return nil, fmt.Errorf("parsing connection URL: %w", err)
		}
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		dsn = u.String()
	}

	log.Infof("connecting to postgres (schema: %s)", schema)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening postgres connection: %w", err)
	}

	// Verify connection
	if err = db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging postgres: %w", err)
	}

	// Create schema if specified
	if schema != "" {
		if err := createSchema(db, schema); err != nil {
			db.Close()
			return nil, err
		}
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	return db, nil
}

// createSchema creates the PostgreSQL schema if it doesn't exist and sets the search path.
func createSchema(db *sql.DB, schema string) error {
	// Create schema if not exists
	_, err := db.Exec(fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema))
	if err != nil {
		return fmt.Errorf("creating schema %s: %w", schema, err)
	}

	// Set search_path for this connection (will be set for each new connection via DSN)
	_, err = db.Exec(fmt.Sprintf("SET search_path TO %s, public", schema))
	if err != nil {
		return fmt.Errorf("setting search_path to %s: %w", schema, err)
	}

	return nil
}
