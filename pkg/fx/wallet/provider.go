package wallet

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/store/keystore"
	"github.com/storacha/piri/pkg/wallet"
)

var Module = fx.Module("wallet",
	fx.Provide(
		fx.Annotate(
			NewWallet,
			fx.As(fx.Self()),
			fx.As(new(wallet.Wallet)),
		),
	),
	fx.Invoke(InitializeWallet),
)

func NewWallet(ks keystore.KeyStore) (*wallet.LocalWallet, error) {
	return wallet.NewWallet(ks)
}

func InitializeWallet(lc fx.Lifecycle, cfg app.PDPServiceConfig, wlt *wallet.LocalWallet) {
	addr := cfg.OwnerAddress
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if has, err := wlt.Has(ctx, addr); err != nil {
				return fmt.Errorf("failed to read wallet for address %s: %w", addr, err)
			} else if !has {
				return fmt.Errorf("wallet for address %s not found, please import with 'piri wallet import ...'", addr)
			}
			return nil
		},
	})
}
