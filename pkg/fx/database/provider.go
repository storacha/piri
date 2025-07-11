package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/database/sqlitedb"
)

var Module = fx.Module("database",
	fx.Provide(
		fx.Annotate(
			ProvideReplicatorDB,
			fx.ResultTags(`name:"replicator_db"`),
		),
	),
)

// ProvideReplicatorDB provides the SQLite database for the replicator job queue
func ProvideReplicatorDB(cfg app.AppConfig) (*sql.DB, error) {
	// If no path is provided (PDP not configured), return nil
	if cfg.Storage.Replicator.DBPath == "" {
		return nil, nil
	}

	// Ensure directory exists
	dir := filepath.Dir(cfg.Storage.Replicator.DBPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	// Create SQLite database connection
	db, err := sqlitedb.New(cfg.Storage.Replicator.DBPath)
	if err != nil {
		return nil, fmt.Errorf("creating replicator database: %w", err)
	}

	return db, nil
}