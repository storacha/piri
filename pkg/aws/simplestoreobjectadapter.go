package aws

import (
	"context"
	"fmt"
	"io"
	"iter"

	"github.com/storacha/go-libstoracha/ipnipublisher/store"

	"github.com/storacha/piri/pkg/store/objectstore"
)

// simpleStoreObjectAdapter adapts a store.SimpleStore to objectstore.ListableStore.
// This enables use of SimpleStore backends (like AWS S3Store) with stores that
// require objectstore.ListableStore (like delegationstore).
type simpleStoreObjectAdapter struct {
	store store.SimpleStore
}

var _ objectstore.ListableStore = (*simpleStoreObjectAdapter)(nil)

// NewSimpleStoreObjectAdapter creates an objectstore.ListableStore adapter for a SimpleStore.
func NewSimpleStoreObjectAdapter(s store.SimpleStore) objectstore.ListableStore {
	return &simpleStoreObjectAdapter{store: s}
}

func (a *simpleStoreObjectAdapter) Put(ctx context.Context, key string, size uint64, data io.Reader) error {
	return a.store.Put(ctx, key, size, data)
}

func (a *simpleStoreObjectAdapter) Get(ctx context.Context, key string, opts ...objectstore.GetOption) (objectstore.Object, error) {
	// Process options but SimpleStore doesn't support range requests
	cfg := objectstore.NewGetConfig()
	cfg.ProcessOptions(opts)
	r := cfg.Range()
	if r.Start != 0 || r.End != nil {
		return nil, fmt.Errorf("SimpleStore adapter does not support range requests")
	}

	body, err := a.store.Get(ctx, key)
	if err != nil {
		if store.IsNotFound(err) {
			return nil, objectstore.ErrNotExist
		}
		return nil, err
	}

	return &simpleStoreObject{body: body}, nil
}

func (a *simpleStoreObjectAdapter) Delete(ctx context.Context, key string) error {
	return fmt.Errorf("SimpleStore adapter does not support Delete")
}

func (a *simpleStoreObjectAdapter) Exists(ctx context.Context, key string) (bool, error) {
	return false, fmt.Errorf("SimpleStore adapter does not support Exists")
}

func (a *simpleStoreObjectAdapter) ListPrefix(ctx context.Context, prefix string) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		yield("", fmt.Errorf("SimpleStore adapter does not support ListPrefix"))
	}
}

// simpleStoreObject wraps an io.ReadCloser as objectstore.Object.
type simpleStoreObject struct {
	body io.ReadCloser
}

func (o *simpleStoreObject) Size() int64 {
	// SimpleStore doesn't provide size information
	return -1
}

func (o *simpleStoreObject) Body() io.ReadCloser {
	return o.body
}
