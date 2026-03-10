package keystore

import (
	"context"
	"fmt"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/ipfs/go-datastore/query"
)

const (
	DatastorePrefix = "keystore/"
	DefaultKeyName  = "default"
)

// KeyInfo is used for storing keys in KeyStore
type KeyInfo struct {
	PrivateKey []byte
}

// KeyStore is used for storing secret keys
type KeyStore interface {
	// Get gets a key out of keystore and returns KeyInfo corresponding to named key
	Get(context.Context, string) (KeyInfo, error)
	// Put saves a key info under given name
	Put(context.Context, string, KeyInfo) error
	// Has returns true if the key exists in the store, false otherwise.
	Has(context.Context, string) (bool, error)
	// List returns a slice of keys in the store
	List(context.Context) ([]KeyInfo, error)
}

type keyStore struct {
	ds datastore.Datastore
}

func NewKeyStore(ds datastore.Datastore) (KeyStore, error) {
	ks := namespace.Wrap(ds, datastore.NewKey(DatastorePrefix))
	return &keyStore{ds: ks}, nil
}

func (k *keyStore) Get(ctx context.Context, s string) (KeyInfo, error) {
	res, err := k.ds.Get(ctx, datastore.NewKey(s))
	if err != nil {
		return KeyInfo{}, fmt.Errorf("getting key (%s): %w", s, err)
	}
	return KeyInfo{PrivateKey: res}, nil
}

func (k *keyStore) Put(ctx context.Context, s string, info KeyInfo) error {
	if err := k.ds.Put(ctx, datastore.NewKey(s), info.PrivateKey); err != nil {
		return fmt.Errorf("putting key (%s): %w", s, err)
	}
	return nil
}

func (k *keyStore) Has(ctx context.Context, s string) (bool, error) {
	has, err := k.ds.Has(ctx, datastore.NewKey(s))
	if err != nil {
		return false, fmt.Errorf("getting key (%s): %w", s, err)
	}
	return has, nil
}

func (k *keyStore) List(ctx context.Context) ([]KeyInfo, error) {
	res, err := k.ds.Query(ctx, query.Query{
		KeysOnly: false,
	})
	if err != nil {
		return nil, fmt.Errorf("listing keys: %w", err)
	}
	out := make([]KeyInfo, 0)
	for entry := range res.Next() {
		out = append(out, KeyInfo{PrivateKey: entry.Value})
	}
	return out, nil
}
