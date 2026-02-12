package flatfs

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/storacha/piri/pkg/store/objectstore"
)

type FlatFSKeyAdapterStore struct {
	store    objectstore.Store
	shardStr string
	getDir   ShardFunc
}

var _ objectstore.Store = (*FlatFSKeyAdapterStore)(nil)

func NewFlatFSKeyAdapter(ctx context.Context, store objectstore.Store, fun *ShardIdV1) (*FlatFSKeyAdapterStore, error) {
	obj, err := store.Get(ctx, SHARDING_FN)
	if err != nil {
		if err != objectstore.ErrNotExist {
			return nil, fmt.Errorf("getting sharding info: %w", err)
		}
		data := fun.String() + "\n"
		err := store.Put(ctx, SHARDING_FN, uint64(len(data)), io.NopCloser(strings.NewReader(data)))
		if err != nil {
			return nil, fmt.Errorf("storing sharding info: %w", err)
		}
	} else {
		body := obj.Body()
		defer body.Close()
		bytes, err := io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("reading sharding info: %w", err)
		}
		id, err := ParseShardFunc(string(bytes))
		if err != nil {
			return nil, fmt.Errorf("parsing existing sharding info: %w", err)
		}
		if id.String() != fun.String() {
			return nil, fmt.Errorf("sharding function mismatch: store has %q but adapter configured with %q", id.String(), fun.String())
		}
	}
	return &FlatFSKeyAdapterStore{
		store:    store,
		shardStr: fun.String(),
		getDir:   fun.Func(),
	}, nil
}

func (f *FlatFSKeyAdapterStore) Delete(ctx context.Context, key string) error {
	// Can't exist in datastore.
	if !keyIsValid(key) {
		return nil
	}
	_, file := f.encode(key)
	return f.store.Delete(ctx, file)
}

func (f *FlatFSKeyAdapterStore) encode(key string) (dir, file string) {
	dir = f.getDir(key)
	file = filepath.Join(dir, key+extension)
	return dir, file
}

func (f *FlatFSKeyAdapterStore) Get(ctx context.Context, key string, opts ...objectstore.GetOption) (objectstore.Object, error) {
	// Can't exist in datastore.
	if !keyIsValid(key) {
		return nil, objectstore.ErrNotExist
	}
	_, file := f.encode(key)
	return f.store.Get(ctx, file, opts...)
}

func (f *FlatFSKeyAdapterStore) Put(ctx context.Context, key string, size uint64, data io.Reader) error {
	if !keyIsValid(key) {
		return fmt.Errorf("when putting %q: %w", key, ErrInvalidKey)
	}
	_, file := f.encode(key)
	return f.store.Put(ctx, file, size, data)
}
