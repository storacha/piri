package app

import (
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/fx/aggregator"
	"github.com/storacha/piri/pkg/fx/blobs"
	"github.com/storacha/piri/pkg/fx/claims"
	"github.com/storacha/piri/pkg/fx/echo"
	"github.com/storacha/piri/pkg/fx/identity"
	"github.com/storacha/piri/pkg/fx/pdp/remotepdp"
	"github.com/storacha/piri/pkg/fx/presigner"
	"github.com/storacha/piri/pkg/fx/principalresolver"
	"github.com/storacha/piri/pkg/fx/publisher"
	"github.com/storacha/piri/pkg/fx/replicator"
	"github.com/storacha/piri/pkg/fx/root"
	"github.com/storacha/piri/pkg/fx/storage"
	"github.com/storacha/piri/pkg/fx/ucan"
)

// FullModule returns the full server module with all components
func UCANServiceModule(cfg app.AppConfig) fx.Option {
	// Collect all modules that should be included
	var modules = []fx.Option{
		// Core infrastructure - always included
		identity.Module, // Provides principal.Signer
		//database.Module,          // Provides SQLite database for job queues
		presigner.Module,         // Provides presigner.RequestPresigner
		root.Module,              // Provides root http handler
		blobs.Module,             // Provides blob service and handler
		claims.Module,            // Provides claims service and handler
		publisher.Module,         // Provides publisher service and handler
		replicator.Module,        // Provides replicator service (works with or without PDP)
		storage.Module,           // Provides storage service wrapper
		principalresolver.Module, // Provides principal resolver for UCAN
		ucan.Module,              // Provides UCAN handler
		echo.Module,              // Provides Echo server with route registration
		aggregator.Module,
	}

	/*
		// Select store module based on whether data directory is configured
		if cfg.Storage.DataDir == "" {
			// Use memory stores for tests or when no data dir is configured
			modules = append(modules, store.MemoryStoreModule)
		} else {
			// Use filesystem stores for production
			modules = append(modules, store.FileSystemStoreModule)
		}

	*/

	// Conditionally include PDP module if configured
	if cfg.Services.PDPServer != nil {
		modules = append(modules, remotepdp.Module)
	}

	return fx.Module("full-server", modules...)
}
