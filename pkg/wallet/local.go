package wallet

import (
	"context"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/storacha/piri/pkg/store/local/keystore"
)

const KNamePrefix = "wallet-"

type Key struct {
	keystore.KeyInfo

	PublicKey []byte
	Address   common.Address
}

func NewKey(keyinfo keystore.KeyInfo) (*Key, error) {
	k := &Key{
		KeyInfo: keyinfo,
	}

	sk, err := crypto.ToECDSA(keyinfo.PrivateKey)
	if err != nil {
		return nil, err
	}
	k.PublicKey = crypto.FromECDSAPub(&sk.PublicKey)
	k.Address = crypto.PubkeyToAddress(sk.PublicKey)

	return k, nil
}

type Wallet interface {
	Import(ctx context.Context, ki *keystore.KeyInfo) (common.Address, error)
	SignTransaction(ctx context.Context, addr common.Address, signer types.Signer, tx *types.Transaction) (*types.Transaction, error)
}

type LocalWallet struct {
	keys     map[common.Address]*Key
	keystore keystore.KeyStore
	keysMu   sync.Mutex
}

func NewWallet(keystore keystore.KeyStore) (*LocalWallet, error) {
	w := &LocalWallet{
		keys:     make(map[common.Address]*Key),
		keystore: keystore,
	}

	return w, nil
}

func (w *LocalWallet) SignTransaction(ctx context.Context, addr common.Address, signer types.Signer, tx *types.Transaction) (*types.Transaction, error) {
	k, err := w.findKey(ctx, addr)
	if err != nil {
		return nil, err
	}
	privateKey, err := crypto.ToECDSA(k.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("converting private key: %w", err)
	}
	return types.SignTx(tx, signer, privateKey)
}

func (w *LocalWallet) Import(ctx context.Context, ki *keystore.KeyInfo) (common.Address, error) {
	w.keysMu.Lock()
	defer w.keysMu.Unlock()

	k, err := NewKey(*ki)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to make key: %w", err)
	}

	if err := w.keystore.Put(ctx, KNamePrefix+k.Address.String(), k.KeyInfo); err != nil {
		return common.Address{}, fmt.Errorf("saving to keystore: %w", err)
	}

	return k.Address, nil
}

func (w *LocalWallet) List(ctx context.Context) ([]*Key, error) {
	w.keysMu.Lock()
	defer w.keysMu.Unlock()

	kis, err := w.keystore.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list wallets: %w", err)
	}

	out := make([]*Key, len(kis))
	for i, k := range kis {
		out[i], err = NewKey(k)
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}

func (w *LocalWallet) Has(ctx context.Context, addr common.Address) (bool, error) {
	w.keysMu.Lock()
	defer w.keysMu.Unlock()

	return w.keystore.Has(ctx, KNamePrefix+addr.String())
}

func (w *LocalWallet) findKey(ctx context.Context, addr common.Address) (*Key, error) {
	w.keysMu.Lock()
	defer w.keysMu.Unlock()

	k, ok := w.keys[addr]
	if ok {
		return k, nil
	}
	if w.keystore == nil {
		return nil, fmt.Errorf("keystore not initialized")
	}

	ki, err := w.keystore.Get(ctx, KNamePrefix+addr.String())
	if err != nil {
		return nil, fmt.Errorf("key not found for address (%s): %w", addr, err)
	}

	k, err = NewKey(ki)
	if err != nil {
		return nil, fmt.Errorf("decoding from keystore for address (%s): %w", addr, err)
	}

	w.keys[k.Address] = k

	return k, nil
}
