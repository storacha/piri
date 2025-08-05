package identity

import (
	"github.com/storacha/go-ucanto/principal"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
)

var Module = fx.Module("identity",
	fx.Provide(ProvideIdentity),
)

// ProvideIdentity extracts the principal signer from the app config
func ProvideIdentity(cfg app.AppConfig) principal.Signer {
	return cfg.Identity.Signer
}
