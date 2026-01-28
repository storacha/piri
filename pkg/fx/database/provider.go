package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/fx"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/database"
	"github.com/storacha/piri/pkg/database/gormdb"
	"github.com/storacha/piri/pkg/database/postgresdb"
	"github.com/storacha/piri/pkg/database/sqlitedb"
)

// PostgreSQL schema names for each logical database
const (
	SchemaReplicator    = "replicator"
	SchemaAggregator    = "aggregator"
	SchemaEgressTracker = "egress_tracker"
	SchemaScheduler     = "scheduler"
)

var Module = fx.Module("database",
	fx.Provide(
		fx.Annotate(
			ProvideReplicatorDB,
			fx.ResultTags(`name:"replicator_db"`),
		),
		fx.Annotate(
			ProvideTaskEngineDB,
			fx.ResultTags(`name:"engine_db"`),
		),
		fx.Annotate(
			ProvideAggregatorDB,
			fx.ResultTags(`name:"aggregator_db"`),
		),
		fx.Annotate(
			ProvideEgressTrackerDB,
			fx.ResultTags(`name:"egress_tracker_db"`),
		),
	),
)

// ProvideReplicatorDB provides the database for the replicator job queue.
// Supports both SQLite (default) and PostgreSQL backends.
func ProvideReplicatorDB(lc fx.Lifecycle, cfg app.StorageConfig) (*sql.DB, error) {
	var db *sql.DB
	var err error

	if cfg.Database.IsPostgres() {
		// Use PostgreSQL with separate schema
		db, err = postgresdb.New(cfg.Database.URL, SchemaReplicator)
		if err != nil {
			return nil, fmt.Errorf("creating postgres replicator database: %w", err)
		}
	} else {
		// Use SQLite (default)
		if cfg.Replicator.DBPath == "" {
			db, err = sqlitedb.NewMemory()
			if err != nil {
				return nil, fmt.Errorf("creating in-memory replicator database: %w", err)
			}
		} else {
			// Ensure directory exists for file-based database
			dir := filepath.Dir(cfg.Replicator.DBPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("creating replicator database directory: %w", err)
			}

			db, err = sqlitedb.New(cfg.Replicator.DBPath,
				database.WithJournalMode(database.JournalModeWAL),
				database.WithTimeout(5*time.Second),
				database.WithSyncMode(database.SyncModeNORMAL),
			)
			if err != nil {
				return nil, fmt.Errorf("creating replicator database: %w", err)
			}
			configureSQLiteConnection(db)
		}
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return db.PingContext(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return db.Close()
		},
	})

	return db, nil
}

// ProvideAggregatorDB provides the database for the aggregator job queue.
// Supports both SQLite (default) and PostgreSQL backends.
func ProvideAggregatorDB(lc fx.Lifecycle, cfg app.StorageConfig) (*sql.DB, error) {
	var db *sql.DB
	var err error

	if cfg.Database.IsPostgres() {
		// Use PostgreSQL with separate schema
		db, err = postgresdb.New(cfg.Database.URL, SchemaAggregator)
		if err != nil {
			return nil, fmt.Errorf("creating postgres aggregator database: %w", err)
		}
	} else {
		// Use SQLite (default)
		if cfg.Aggregator.DBPath == "" {
			db, err = sqlitedb.NewMemory()
			if err != nil {
				return nil, fmt.Errorf("creating in-memory aggregator database: %w", err)
			}
		} else {
			// Ensure directory exists for file-based database
			dir := filepath.Dir(cfg.Aggregator.DBPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("creating aggregator database directory: %w", err)
			}

			db, err = sqlitedb.New(cfg.Aggregator.DBPath,
				database.WithJournalMode(database.JournalModeWAL),
				database.WithTimeout(5*time.Second),
				database.WithSyncMode(database.SyncModeNORMAL),
			)
			if err != nil {
				return nil, fmt.Errorf("creating aggregator database: %w", err)
			}
			configureSQLiteConnection(db)
		}
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return db.PingContext(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return db.Close()
		},
	})

	return db, nil
}

// ProvideTaskEngineDB provides the GORM database for the task engine scheduler.
// Supports both SQLite (default) and PostgreSQL backends.
func ProvideTaskEngineDB(lc fx.Lifecycle, cfg app.StorageConfig) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	if cfg.Database.IsPostgres() {
		// Use PostgreSQL with separate schema
		db, err = gormdb.NewPostgres(cfg.Database.URL, SchemaScheduler, nil)
		if err != nil {
			return nil, fmt.Errorf("creating postgres task engine db: %w", err)
		}
	} else {
		// Use SQLite (default)
		dbPath := cfg.SchedulerStorage.DBPath
		dbOpts := []database.Option{
			// ensure foreign key constraints are respected.
			database.WithForeignKeyConstraintsEnable(true),
			// wait up to 5 seconds before failing to write due to busted database.
			database.WithTimeout(5 * time.Second),
		}
		if dbPath == "" {
			dbPath = "file::memory:?cache=shared"
			// use an in-memory cache for in-memory database
			dbOpts = append(dbOpts, database.WithJournalMode(database.JournalModeMEMORY))
		} else {
			// use a write ahead log for transactions, good for parallel operations on persisted databases
			dbOpts = append(dbOpts, database.WithJournalMode(database.JournalModeWAL))
		}

		db, err = gormdb.New(dbPath, dbOpts...)
		if err != nil {
			return nil, fmt.Errorf("creating task engine db: %w", err)
		}

		// Ensure single connection for SQLite to prevent locking issues
		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("getting underlying sql.DB: %w", err)
		}
		configureSQLiteConnection(sqlDB)
	}

	lc.Append(fx.Hook{
		// NB(forrest): we don't ping the gorm database on startup since the gorm package does so internally.
		OnStop: func(ctx context.Context) error {
			ddb, err := db.DB()
			if err != nil {
				return fmt.Errorf("stopping task engine db: %w", err)
			}
			if err := ddb.Close(); err != nil {
				return fmt.Errorf("stopping task engine db: %w", err)
			}
			return nil
		},
	})
	return db, nil
}

// ProvideEgressTrackerDB provides the database for the egress tracker job queue.
// Supports both SQLite (default) and PostgreSQL backends.
func ProvideEgressTrackerDB(lc fx.Lifecycle, cfg app.StorageConfig) (*sql.DB, error) {
	var db *sql.DB
	var err error

	if cfg.Database.IsPostgres() {
		// Use PostgreSQL with separate schema
		db, err = postgresdb.New(cfg.Database.URL, SchemaEgressTracker)
		if err != nil {
			return nil, fmt.Errorf("creating postgres egress tracker database: %w", err)
		}
	} else {
		// Use SQLite (default)
		if cfg.EgressTracker.DBPath == "" {
			db, err = sqlitedb.NewMemory()
			if err != nil {
				return nil, fmt.Errorf("creating in-memory egress tracker database: %w", err)
			}
		} else {
			// Ensure directory exists for file-based database
			dir := filepath.Dir(cfg.EgressTracker.DBPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("creating egress tracker database directory: %w", err)
			}

			db, err = sqlitedb.New(cfg.EgressTracker.DBPath,
				database.WithJournalMode(database.JournalModeWAL),
				database.WithTimeout(5*time.Second),
				database.WithSyncMode(database.SyncModeNORMAL),
			)
			if err != nil {
				return nil, fmt.Errorf("creating egress tracker database: %w", err)
			}
			configureSQLiteConnection(db)
		}
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return db.PingContext(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return db.Close()
		},
	})

	return db, nil
}

// configureSQLiteConnection configures a SQLite database connection with appropriate limits.
// SQLite only supports a single writer, so we limit connections to prevent locking issues.
func configureSQLiteConnection(db *sql.DB) {
	// there can only be ONE connection or sqlite throws a massive tantrum about the
	// database being locked...sobs...wipes tears with mouse pad...
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0) // Don't expire the connection
}
