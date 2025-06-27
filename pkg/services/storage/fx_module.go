package storage

import (
	"go.uber.org/fx"
)

// ServiceModule provides the storage orchestrator service.
// This service coordinates between blob, claim, publisher, and replicator services.
var ServiceModule = fx.Module("storage-service",
	fx.Provide(NewService),
)
