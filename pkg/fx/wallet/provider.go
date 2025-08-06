package wallet

import (
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/store/keystore"
	"github.com/storacha/piri/pkg/wallet"
)

var Module = fx.Module("wallet",
	fx.Provide(
		NewWallet,
	),
)

func NewWallet(ks keystore.KeyStore) (wallet.Wallet, error) {
	return wallet.NewWallet(ks)
}
