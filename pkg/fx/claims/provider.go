package claims

import (
	"go.uber.org/fx"

	echofx "github.com/storacha/piri/pkg/fx/echo"
	"github.com/storacha/piri/pkg/service/claims"
	publisherSvc "github.com/storacha/piri/pkg/service/publisher"
	"github.com/storacha/piri/pkg/store/claimstore"
)

var Module = fx.Module("claims",
	fx.Provide(
		// Also provide the interface
		fx.Annotate(
			NewService,
			fx.As(new(claims.Claims)),
		),
		fx.Annotate(
			claims.NewServer,
			fx.As(new(echofx.RouteRegistrar)),
			fx.ResultTags(`group:"route_registrar"`),
		),
	),
)

func NewService(
	claimStore claimstore.ClaimStore,
	pub publisherSvc.Publisher,
) *claims.ClaimService {
	return claims.NewV2(claimStore, pub)
}
