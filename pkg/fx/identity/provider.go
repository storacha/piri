package identity

import (
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/ucan"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
)

var Module = fx.Module("identity",
	fx.Provide(
		fx.Annotate(
			ProvideIdentity,
			fx.As(fx.Self()),
			fx.As(new(ucan.Signer)),
		),
	),
)

// ProvideIdentity extracts the principal signer from the app config
func ProvideIdentity(cfg app.AppConfig) principal.Signer {
	return cfg.Identity.Signer
}
