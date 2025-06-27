package services

import (
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/services/blob"
	"github.com/storacha/piri/pkg/services/claim"
	"github.com/storacha/piri/pkg/services/pdp"
	"github.com/storacha/piri/pkg/services/publisher"
	"github.com/storacha/piri/pkg/services/replicator"
	"github.com/storacha/piri/pkg/services/storage"
)

// ServiceModule provides all service implementations without HTTP handlers, UCAN service methods,
// or datastores.
// This module expects datastores and configuration to be provided separately.
var ServiceModule = fx.Module("services-impls",
	// Individual service modules
	blob.ServiceModule,
	claim.ServiceModule,
	publisher.ServiceModule,
	storage.ServiceModule,
	replicator.ServiceModule,
)

// HTTPHandlersModule provides all HTTP handler modules.
// This module expects services to be provided separately.
var HTTPHandlersModule = fx.Module("services-http",
	blob.HTTPModule,
	claim.HTTPModule,
	publisher.HTTPModule,
)

// UCANMethodsModule provides all UCAN service method modules.
// This module expects services to be provided separately.
var UCANMethodsModule = fx.Module("ucan-methods",
	blob.UCANModule,
	pdp.UCANModule,
	replicator.UCANModule,
)
