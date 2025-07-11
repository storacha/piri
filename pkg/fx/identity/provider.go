package identity

import (
	"fmt"

	"github.com/storacha/go-ucanto/principal"
	"go.uber.org/fx"

	"github.com/storacha/piri/cmd/cliutil"
	"github.com/storacha/piri/pkg/config/app"
)

var Module = fx.Module("identity",
	fx.Provide(NewIdentity),
)

// NewIdentity creates a principal signer from the configured key file
func NewIdentity(cfg app.AppConfig) (principal.Signer, error) {
	id, err := cliutil.ReadPrivateKeyFromPEM(cfg.Identity.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("loading principal signer: %w", err)
	}
	return id, nil
}
