package ipldstore

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-ucanto/core/ipld/codec/cbor"
)

type KVStore[K, V any] interface {
	Get(ctx context.Context, key K) (V, error)
	Put(ctx context.Context, key K, value V) error
}

type ipldStore[K fmt.Stringer, V any] struct {
	ds   store.SimpleStore
	typ  schema.Type
	opts []bindnode.Option
}

func (i *ipldStore[K, V]) Get(ctx context.Context, key K) (V, error) {
	var zeroV V
	r, err := i.ds.Get(ctx, key.String())
	if err != nil {
		return zeroV, err
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		return zeroV, err
	}
	var v V
	err = cbor.Decode(data, &v, i.typ, i.opts...)
	if err != nil {
		return zeroV, err
	}
	return v, nil
}

func (i *ipldStore[K, V]) Put(ctx context.Context, key K, value V) error {
	data, err := cbor.Encode(&value, i.typ, i.opts...)
	if err != nil {
		return err
	}
	return i.ds.Put(ctx, key.String(), uint64(len(data)), bytes.NewReader(data))
}

func IPLDStore[K fmt.Stringer, V any](ds store.SimpleStore, typ schema.Type, opts ...bindnode.Option) KVStore[K, V] {
	return &ipldStore[K, V]{
		ds:   ds,
		typ:  typ,
		opts: opts,
	}
}
