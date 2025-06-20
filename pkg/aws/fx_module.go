// Package aws provides fx modules for AWS-backed storage services.
//
// This package provides AWS-specific datastore implementations that compose
// with the service modules from pkg/services/. The modules follow fx best
// practices by providing only the datastore implementations, allowing the
// service logic to be reused from pkg/services/.
//
// Module architecture:
//
//	┌──────────────────────────────────────────────────────────────┐
//	│                      aws.ServerModule                        │
//	│  ┌───────────────────────────────────────────────────────────┤
//	│  │                      aws.Module                            │
//	│  │  ┌─────────────────┐ ┌─────────────────┐ ┌──────────────┐│
//	│  │  │aws.DatastoreModule│ │aws.ConfigModule │ │storage.      ││
//	│  │  │                 │ │                 │ │ServiceModule ││
//	│  │  │- S3 Blobstore   │ │- Public URLs    │ │              ││
//	│  │  │- DynamoDB       │ │- Service Conns  │ │- blob.Module ││
//	│  │  │  Allocation     │ │- Presigner      │ │- claim.Module││
//	│  │  │- S3 Claims      │ │- PDP Service    │ │- publisher.  ││
//	│  │  │- S3 Publisher   │ │- Principal      │ │  Module      ││
//	│  │  │- S3 Receipts    │ │- IPNI URLs      │ │- replicator. ││
//	│  │  └─────────────────┘ └─────────────────┘ │  Module      ││
//	│  │                                           │- storage.    ││
//	│  │                                           │  NewService  ││
//	│  │                                           └──────────────┘│
//	│  └───────────────────────────────────────────────────────────┤
//	│  ┌───────────────────────────────────────────────────────────┤
//	│  │              storage.HTTPServerModule                     │
//	│  │  - blobserver.Module                                      │
//	│  │  - claimserver.Module                                     │
//	│  │  - publisherserver.Module                                 │
//	│  │  - ucan.Module                                            │
//	│  │  - server.Module                                          │
//	│  └───────────────────────────────────────────────────────────┤
//	└──────────────────────────────────────────────────────────────┘
//
// Usage example:
//
//	app := fx.New(
//	    // Use AWS-backed storage with all servers
//	    aws.ServerModule,
//
//	    // Provide Echo server
//	    fx.Provide(func() *echo.Echo {
//	        e := echo.New()
//	        return e
//	    }),
//
//	    // Start server
//	    fx.Invoke(startServer),
//	)
package aws

import (
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/services"
)

// Note: AWS datastore providers have been moved to pkg/datastores.AWSModule
// to ensure consistency with filesystem and memory datastore modules.
// Use datastores.AWSModule when composing your fx app.

// ConfigModule provides AWS-specific configuration and external connections.
// This module handles configuration transformation and service connections
// specific to AWS deployments.
var ConfigModule = fx.Module("aws-config",
	// AWS Config from environment
	fx.Provide(ProvideConfig),
	
	// Configuration transformers and external connections
	fx.Provide(
		ProvideAWSPublicURL,
		ProvideAWSBlobsPublicURL,
		ProvideAWSIndexingServiceDID,
		ProvideAWSIndexingServiceURL,
		ProvideAWSIndexingServiceProofs,
		ProvideAWSPeerMultiAddress,
		ProvideAWSBlobsAccess,
		ProvideAWSBlobsPreSigner,
		ProvideAWSPDPService,
		ProvidePrincipalSigner,
		ProvideIPNIAnnounceURLs,
		ProvideServicePrincipalMapping,
		ProvideUploadServiceDID,
		ProvideUploadServiceURL,
		fx.Annotate(
			ProvideUploadServiceConnection,
			fx.ResultTags(`name:"upload"`),
		),
		fx.Annotate(
			ProvideIndexingServiceConnection,
			fx.ResultTags(`name:"indexing"`),
		),
	),
)

// Module composes AWS configuration with service logic from pkg/services/.
// This module is used by the Construct function to build services for Lambda handlers.
// Note: This module does not include datastores - use it with datastores.AWSModule.
var Module = fx.Module("aws-service",
	// Include AWS configuration
	ConfigModule,

	// Include service logic from pkg/services
	// This provides all service constructors (blob, claim, publisher, replicator)
	// and the main storage service constructor
	services.ServiceModule,
)

// Usage example for Lambda functions:
//
//	func Construct(cfg Config) (types.Service, error) {
//	    app := fx.New(
//	        fx.Supply(cfg),
//	        aws.ConfigModule,
//	        datastores.AWSModule,
//	        services.ServiceModule,
//	        fx.Invoke(func(s types.Service) { service = s }),
//	    )
//	    return service, app.Err()
//	}
