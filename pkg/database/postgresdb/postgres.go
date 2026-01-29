package postgresdb

import (
	"database/sql"
	"fmt"
	"net/url"
	"time"

	logging "github.com/ipfs/go-log/v2"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/storacha/piri/pkg/config/app"
)

var log = logging.Logger("database")

const (
	// DefaultMaxOpenConns is the default maximum number of open connections.
	// Conservative default: 4 database pools × 5 = 20 total connections,
	// well under PostgreSQL's default max_connections of 100.
	DefaultMaxOpenConns = 5
	// DefaultMaxIdleConns is the default maximum number of idle connections.
	// Set equal to MaxOpenConns to avoid connection churn.
	DefaultMaxIdleConns = 5
	// DefaultConnMaxLifetime is the default maximum connection lifetime.
	// 30 minutes provides stability while still recycling stale connections.
	DefaultConnMaxLifetime = 30 * time.Minute
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

	// If schema is specified, append search_path to connection string.
	// We include 'public' in the search_path so that built-in functions
	// like gen_random_uuid() are accessible.
	dsn := connURL
	if schema != "" {
		u, err := url.Parse(connURL)
		if err != nil {
			return nil, fmt.Errorf("parsing connection URL: %w", err)
		}
		q := u.Query()
		q.Set("search_path", fmt.Sprintf("%s,public", schema))
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

// createSchema creates the PostgreSQL schema if it doesn't exist.
// Note: search_path is already set via the DSN for all connections.
func createSchema(db *sql.DB, schema string) error {
	_, err := db.Exec(fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema))
	if err != nil {
		return fmt.Errorf("creating schema %s: %w", schema, err)
	}
	return nil
}

// OptionsFromConfig creates functional options from app.PoolConfig.
// Returns nil if cfg is nil. Zero values in cfg use the defaults.
func OptionsFromConfig(cfg app.PostgresConfig) []Option {
	var opts []Option
	if cfg.MaxOpenConns > 0 {
		opts = append(opts, WithMaxOpenConns(cfg.MaxOpenConns))
	}
	if cfg.MaxIdleConns > 0 {
		opts = append(opts, WithMaxIdleConns(cfg.MaxIdleConns))
	}
	if cfg.ConnMaxLifetime > 0 {
		opts = append(opts, WithConnMaxLifetime(cfg.ConnMaxLifetime))
	}
	return opts
}
