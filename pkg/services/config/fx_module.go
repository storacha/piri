package config

import (
	"go.uber.org/fx"
)

// Module provides configuration transformers and external service connections.
// This module transforms configuration into service dependencies that are
// shared across all storage implementations (filesystem, AWS, memory, etc).
//
// Includes:
// - URL/Address converters (presigner, access, multiaddr)
// - External service connections (upload, indexing)
// - Configuration extractors (proofs, announce URLs)
// - Optional services (PDP)
var Module = fx.Module("services-config",
	fx.Provide(
		ProvidePrincipal,
		ProvidePublicURLPreSigner,
		ProvidePublicURLAccess,
		ProvidePeerMultiAddress,
		fx.Annotate(
			ProvideUploadServiceConnection,
			fx.ResultTags(`name:"upload"`),
		),
		ProvidePDPService,
		ProvideIndexingServiceProofs,
		ProvideIPNIAnnounceURLs,
		fx.Annotate(
			ProvideIndexingServiceConnection,
			fx.ResultTags(`name:"indexing"`),
		),
		ProvideServicePrincipalMapping,
		fx.Annotate(
			ProvideUploadServiceDID,
			fx.ResultTags(`name:"upload"`),
		),
		fx.Annotate(
			ProvideUploadServiceURL,
			fx.ResultTags(`name:"upload"`),
		),
		fx.Annotate(
			ProvideIndexingServiceDID,
			fx.ResultTags(`name:"indexing"`),
		),
		fx.Annotate(
			ProvideIndexingServiceURL,
			fx.ResultTags(`name:"indexing"`),
		),
	),
)
