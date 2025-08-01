package blobs

import (
	"fmt"

	"github.com/storacha/go-ucanto/principal"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/access"
	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/presigner"
	"github.com/storacha/piri/pkg/service/blobs"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
)

var Module = fx.Module("blobs",
	fx.Provide(
		NewService,
		// Also provide the interface
		fx.Annotate(
			func(svc *blobs.BlobService) blobs.Blobs {
				return svc
			},
		),
		fx.Annotate(
			blobs.NewServer,
			fx.ResultTags(`group:"route_registrar"`),
		),
	),
)

func NewService(
	cfg app.AppConfig,
	id principal.Signer,
	ps presigner.RequestPresigner,
	blobStore blobstore.Blobstore,
	allocationStore allocationstore.AllocationStore,
) (*blobs.BlobService, error) {
	if cfg.Server.PublicURL == nil {
		return nil, fmt.Errorf("public URL required for blob service")
	}

	if !id.DID().Defined() {
		return nil, fmt.Errorf("invalid DID for blob service")
	}

	accessURL := cfg.Server.PublicURL
	accessURL.Path = "/blob"
	ap, err := access.NewPatternAccess(fmt.Sprintf("%s/{blob}", accessURL.String()))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize access pattern for blob service: %w", err)
	}

	return blobs.New(
		blobs.WithAccess(ap),
		blobs.WithPresigner(ps),
		blobs.WithBlobstore(blobStore),
		blobs.WithAllocationStore(allocationStore),
	)
}
