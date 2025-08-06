package database

import (
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
func ProvideReplicatorDB(cfg app.AppConfig) (*sql.DB, error) {
	// If no path is provided, use in-memory database
	if cfg.Storage.Replicator.DBPath == "" {
		db, err := sqlitedb.NewMemory()
		if err != nil {
			return nil, fmt.Errorf("creating in-memory replicator database: %w", err)
		}
		return db, nil
	}

	// Ensure directory exists for file-based database
	dir := filepath.Dir(cfg.Storage.Replicator.DBPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating replicator database directory: %w", err)
	}

	// Create SQLite database connection
	db, err := sqlitedb.New(cfg.Storage.Replicator.DBPath)
	if err != nil {
		return nil, fmt.Errorf("creating replicator database: %w", err)
	}

	return db, nil
}

func ProviderAggregatorDB(cfg app.AppConfig) (*sql.DB, error) {
	// If no path is provided, use in-memory database
	if cfg.Storage.Aggregator.DBPath == "" {
		db, err := sqlitedb.NewMemory()
		if err != nil {
			return nil, fmt.Errorf("creating in-memory aggregator database: %w", err)
		}
		return db, nil
	}

	// Ensure directory exists for file-based database
	dir := filepath.Dir(cfg.Storage.Aggregator.DBPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating aggregator database directory: %w", err)
	}

	// Create SQLite database connection
	db, err := sqlitedb.New(cfg.Storage.Aggregator.DBPath,
		database.WithJournalMode(database.JournalModeWAL),
		database.WithTimeout(5*time.Second),
		database.WithSyncMode(database.SyncModeNORMAL),
	)
	if err != nil {
		return nil, fmt.Errorf("creating aggregator database: %w", err)
	}

	return db, nil

}

func ProvideTaskEngineDB(cfg app.AppConfig) (*gorm.DB, error) {
	dbPath := cfg.Storage.SchedulerStorage.DBPath
	if dbPath == "" {
		dbPath = "file::memory:?cache=shared"
	}

	return gormdb.New(dbPath,
		// use a write ahead log for transactions, good for parallel operations.
		database.WithJournalMode(database.JournalModeWAL),
		// ensure foreign key constraints are respected.
		database.WithForeignKeyConstraintsEnable(true),
		// wait up to 5 seconds before failing to write due to busted database.
		database.WithTimeout(5*time.Second),
	)
}
