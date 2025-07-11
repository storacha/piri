package app

import (
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/fx/blobs"
	"github.com/storacha/piri/pkg/fx/claims"
	"github.com/storacha/piri/pkg/fx/config"
	"github.com/storacha/piri/pkg/fx/database"
	"github.com/storacha/piri/pkg/fx/echo"
	"github.com/storacha/piri/pkg/fx/identity"
	"github.com/storacha/piri/pkg/fx/pdp"
	"github.com/storacha/piri/pkg/fx/principalresolver"
	"github.com/storacha/piri/pkg/fx/publisher"
	"github.com/storacha/piri/pkg/fx/replicator"
	"github.com/storacha/piri/pkg/fx/storage"
	"github.com/storacha/piri/pkg/fx/store"
	"github.com/storacha/piri/pkg/fx/ucan"
)

// FullModule contains all modules needed for the full server
var FullModule = fx.Module("full-server",
	// Core infrastructure
	config.Module,   // Provides configuration values
	identity.Module, // Provides principal.Signer
	store.Module,    // Provides all datastores (filesystem-based)
	database.Module, // Provides SQLite database for job queues
	pdp.Module,      // Provides PDP service (or nil if not configured)

	// Services
	blobs.Module,             // Provides blob service and handler
	claims.Module,            // Provides claims service and handler (includes publisher module)
	publisher.Module,         // Provides publisher service and handler
	replicator.Module,        // Provides replicator service
	storage.Module,           // Provides storage service wrapper
	principalresolver.Module, // Provides principal resolver for UCAN

	// HTTP/UCAN
	ucan.Module, // Provides UCAN handler
	echo.Module, // Provides Echo server with route registration
)
