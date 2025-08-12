package replicator

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/storacha/go-ucanto/principal"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/service/blobs"
	"github.com/storacha/piri/pkg/service/claims"
	"github.com/storacha/piri/pkg/service/replicator"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

var Module = fx.Module("replicator",
	fx.Provide(
		fx.Annotate(
			New,
			fx.As(new(replicator.Replicator)),
		),
	),
)

type Params struct {
	fx.In

	Config       app.AppConfig
	ID           principal.Signer
	PDP          pdp.PDP `optional:"true"`
	Blobs        blobs.Blobs
	Claims       claims.Claims
	ReceiptStore receiptstore.ReceiptStore
	DB           *sql.DB `name:"replicator_db"`
}

func New(params Params, lc fx.Lifecycle) (*replicator.Service, error) {
	r, err := replicator.New(
		params.ID,
		params.PDP,
		params.Blobs,
		params.Claims,
		params.ReceiptStore,
		params.Config.UCANService.Services.Upload.Connection,
		params.DB,
	)
	if err != nil {
		return nil, fmt.Errorf("new replicator: %w", err)
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// NB(forrest): accept a context todo since the lifecycle hook context has a timeout of 10 sec for starting
			// a service, we don't want the replicator to timeout here.
			// Long term fix is for replicator start to accept a contex scoped only for startup operation, or none at all
			// since it has a dedicated stop method
			return r.Start(context.TODO())
		},
		OnStop: func(ctx context.Context) error {
			return r.Stop(ctx)
		},
	})

	return r, nil
}
