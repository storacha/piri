package blobs

import (
	"fmt"

	"github.com/storacha/go-ucanto/principal"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/access"
	"github.com/storacha/piri/pkg/config/app"
	echofx "github.com/storacha/piri/pkg/fx/echo"
	"github.com/storacha/piri/pkg/presigner"
	"github.com/storacha/piri/pkg/service/blobs"
	"github.com/storacha/piri/pkg/store/acceptancestore"
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
			fx.As(new(echofx.RouteRegistrar)),
			fx.ResultTags(`group:"route_registrar"`),
		),
	),
)

type NewServiceParams struct {
	fx.In

	Cfg             app.AppConfig
	ID              principal.Signer
	PS              presigner.RequestPresigner
	BlobStore       blobstore.Blobstore
	AllocationStore allocationstore.AllocationStore
	AcceptanceStore acceptancestore.AcceptanceStore
}

func NewService(params NewServiceParams) (*blobs.BlobService, error) {
	if params.Cfg.Server.PublicURL.Scheme == "" {
		return nil, fmt.Errorf("public URL required for blob service")
	}

	if !params.ID.DID().Defined() {
		return nil, fmt.Errorf("invalid DID for blob service")
	}

	accessURL := params.Cfg.Server.PublicURL
	accessURL.Path = "/blob"
	ap, err := access.NewPatternAccess(fmt.Sprintf("%s/{blob}", accessURL.String()))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize access pattern for blob service: %w", err)
	}

	return blobs.New(
		blobs.WithAccess(ap),
		blobs.WithPresigner(params.PS),
		blobs.WithBlobstore(params.BlobStore),
		blobs.WithAllocationStore(params.AllocationStore),
		blobs.WithAcceptanceStore(params.AcceptanceStore),
	)
}
