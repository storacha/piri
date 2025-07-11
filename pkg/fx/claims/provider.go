package claims

import (
	"go.uber.org/fx"
	
	"github.com/storacha/piri/pkg/service/claims"
	publisherSvc "github.com/storacha/piri/pkg/service/publisher"
	"github.com/storacha/piri/pkg/store/claimstore"
)

var Module = fx.Module("claims",
	fx.Provide(
		NewService,
		fx.Annotate(
			NewHandler,
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
