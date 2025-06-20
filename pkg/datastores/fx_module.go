package datastores

import (
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/datastores/filesystem"
	"github.com/storacha/piri/pkg/datastores/memory"
)

// FilesystemModule provides filesystem-based datastore implementations.
// This module provides all the datastores needed by the storage services.
var FilesystemModule = fx.Module("filesystem-datastores",
	fx.Provide(
		filesystem.ProvideBlobstore,
		filesystem.ProvideAllocationStore,
		filesystem.ProvideClaimsStore,
		filesystem.ProvidePublisherStore,
		filesystem.ProvideReceiptStore,
	),
)

// MemoryModule provides in-memory datastore implementations.
// This module is useful for testing and development.
var MemoryModule = fx.Module("memory-datastores",
	fx.Provide(
		memory.ProvideBlobstore,
		memory.ProvideAllocationStore,
		memory.ProvideClaimsStore,
		memory.ProvidePublisherStore,
		memory.ProvideReceiptStore,
	),
)

// Note: AWS datastore providers are available in pkg/aws package.
// They are not included here to avoid circular dependencies.
// When using AWS, include the providers directly in your fx app:
//
//	fx.Provide(
//	    aws.ProvideAWSBlobstore,
//	    aws.ProvideAWSAllocationStore,
//	    aws.ProvideAWSClaimStore,
//	    aws.ProvideAWSPublisherStore,
//	    aws.ProvideAWSReceiptStore,
//	)
