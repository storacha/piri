package keystore

import (
	"context"
	"fmt"
)

func NewMemKeyStore() *MemKeyStore {
	return &MemKeyStore{
		make(map[string]KeyInfo),
	}
}

type MemKeyStore struct {
	m map[string]KeyInfo
}

var _ KeyStore = (*MemKeyStore)(nil)

// Get gets a key out of keystore and returns KeyInfo corresponding to named key
func (mks *MemKeyStore) Get(ctx context.Context, k string) (KeyInfo, error) {
	ki, ok := mks.m[k]
	if !ok {
		return KeyInfo{}, fmt.Errorf("key not found")
	}

	return ki, nil
}

// Put saves a key info under given name
func (mks *MemKeyStore) Put(ctx context.Context, k string, ki KeyInfo) error {
	mks.m[k] = ki
	return nil
}

func (mks *MemKeyStore) Has(ctx context.Context, s string) (bool, error) {
	_, has := mks.m[s]
	return has, nil
}

// List lists all the keys stored in the KeyStore
func (mks *MemKeyStore) List(ctx context.Context) ([]KeyInfo, error) {
	var out []KeyInfo
	for _, ki := range mks.m {
		out = append(out, ki)
	}
	return out, nil
}
