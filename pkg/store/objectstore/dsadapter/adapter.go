package dsadapter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"

	"github.com/storacha/piri/pkg/store/objectstore"
)

// Adapter wraps a datastore.Datastore to implement objectstore.ListableStore.
type Adapter struct {
	ds datastore.Datastore
}

var _ objectstore.ListableStore = (*Adapter)(nil)

// New creates a new Adapter wrapping the given datastore.
func New(ds datastore.Datastore) *Adapter {
	return &Adapter{ds: ds}
}

func (a *Adapter) Put(ctx context.Context, key string, size uint64, data io.Reader) error {
	value, err := io.ReadAll(data)
	if err != nil {
		return fmt.Errorf("reading data: %w", err)
	}
	if uint64(len(value)) != size {
		return fmt.Errorf("size mismatch: expected %d, got %d", size, len(value))
	}
	return a.ds.Put(ctx, datastore.NewKey(key), value)
}

func (a *Adapter) Get(ctx context.Context, key string, opts ...objectstore.GetOption) (objectstore.Object, error) {
	cfg := objectstore.NewGetConfig()
	cfg.ProcessOptions(opts)

	value, err := a.ds.Get(ctx, datastore.NewKey(key))
	if err != nil {
		if errors.Is(err, datastore.ErrNotFound) {
			return nil, objectstore.ErrNotExist
		}
		return nil, fmt.Errorf("getting key %s: %w", key, err)
	}

	// Handle range requests - only validate if range is specified
	r := cfg.Range()
	rangeSpecified := r.Start != 0 || r.End != nil
	if rangeSpecified {
		size := len(value)
		start := int(r.Start)

		// Validate range satisfiability
		if start >= size {
			return nil, objectstore.ErrRangeNotSatisfiable{Range: r}
		}
		if r.End != nil {
			// End less than Start is invalid
			if *r.End < r.Start {
				return nil, objectstore.ErrRangeNotSatisfiable{Range: r}
			}
			// End beyond blob size is invalid
			if int(*r.End) >= size {
				return nil, objectstore.ErrRangeNotSatisfiable{Range: r}
			}
		}
	}

	return &dsObject{
		data:         value,
		originalSize: int64(len(value)),
		byteRange:    r,
	}, nil
}

func (a *Adapter) Delete(ctx context.Context, key string) error {
	return a.ds.Delete(ctx, datastore.NewKey(key))
}

func (a *Adapter) Exists(ctx context.Context, key string) (bool, error) {
	return a.ds.Has(ctx, datastore.NewKey(key))
}

func (a *Adapter) ListPrefix(ctx context.Context, prefix string) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		results, err := a.ds.Query(ctx, query.Query{
			Prefix:   prefix,
			KeysOnly: true,
		})
		if err != nil {
			yield("", fmt.Errorf("querying prefix %s: %w", prefix, err))
			return
		}
		defer results.Close()

		for entry := range results.Next() {
			if entry.Error != nil {
				yield("", fmt.Errorf("iterating results: %w", entry.Error))
				return
			}
			// Remove leading slash from datastore key
			key := entry.Key
			if len(key) > 0 && key[0] == '/' {
				key = key[1:]
			}
			if !yield(key, nil) {
				return
			}
		}
	}
}

// dsObject implements objectstore.Object for datastore values.
type dsObject struct {
	data         []byte
	originalSize int64
	byteRange    objectstore.Range
}

func (o *dsObject) Size() int64 {
	return o.originalSize
}

func (o *dsObject) Body() io.ReadCloser {
	b := o.data
	start := int(o.byteRange.Start)
	end := len(b)
	if o.byteRange.End != nil {
		end = int(*o.byteRange.End + 1) // End is inclusive
	}
	return io.NopCloser(bytes.NewReader(b[start:end]))
}
