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
	"github.com/storacha/piri/pkg/database/sqlitedb"
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
			ProviderAggregatorDB,
			fx.ResultTags(`name:"aggregator_db"`),
		),
	),
)

// ProvideReplicatorDB provides the SQLite database for the replicator job queue
func ProvideReplicatorDB(lc fx.Lifecycle, cfg app.StorageConfig) (*sql.DB, error) {
	// If no path is provided, use in-memory database
	if cfg.Replicator.DBPath == "" {
		db, err := sqlitedb.NewMemory()
		if err != nil {
			return nil, fmt.Errorf("creating in-memory replicator database: %w", err)
		}
		return db, nil
	}

	// Ensure directory exists for file-based database
	dir := filepath.Dir(cfg.Replicator.DBPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating replicator database directory: %w", err)
	}

	// Create SQLite database connection
	db, err := sqlitedb.New(cfg.Replicator.DBPath,
		database.WithJournalMode(database.JournalModeWAL),
		database.WithTimeout(5*time.Second),
		database.WithSyncMode(database.SyncModeNORMAL),
	)
	if err != nil {
		return nil, fmt.Errorf("creating replicator database: %w", err)
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

func ProviderAggregatorDB(lc fx.Lifecycle, cfg app.StorageConfig) (*sql.DB, error) {
	// If no path is provided, use in-memory database
	if cfg.Aggregator.DBPath == "" {
		db, err := sqlitedb.NewMemory()
		if err != nil {
			return nil, fmt.Errorf("creating in-memory aggregator database: %w", err)
		}
		return db, nil
	}

	// Ensure directory exists for file-based database
	dir := filepath.Dir(cfg.Aggregator.DBPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating aggregator database directory: %w", err)
	}

	// Create SQLite database connection
	db, err := sqlitedb.New(cfg.Aggregator.DBPath,
		database.WithJournalMode(database.JournalModeWAL),
		database.WithTimeout(5*time.Second),
		database.WithSyncMode(database.SyncModeNORMAL),
	)
	if err != nil {
		return nil, fmt.Errorf("creating aggregator database: %w", err)
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

func ProvideTaskEngineDB(lc fx.Lifecycle, cfg app.StorageConfig) (*gorm.DB, error) {
	dbPath := cfg.SchedulerStorage.DBPath
	if dbPath == "" {
		dbPath = "file::memory:?cache=shared"
	}

	db, err := gormdb.New(dbPath,
		// use a write ahead log for transactions, good for parallel operations.
		database.WithJournalMode(database.JournalModeWAL),
		// ensure foreign key constraints are respected.
		database.WithForeignKeyConstraintsEnable(true),
		// wait up to 5 seconds before failing to write due to busted database.
		database.WithTimeout(5*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("creating task engine db: %w", err)
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ddb, err := db.DB()
			if err != nil {
				return fmt.Errorf("starting task engine db: %w", err)
			}
			if err := ddb.PingContext(ctx); err != nil {
				return fmt.Errorf("starting task engine db: %w", err)
			}
			return nil
		},
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
