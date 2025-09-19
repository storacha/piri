package blobstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/ipfs/go-datastore"
	"github.com/multiformats/go-multihash"

	"github.com/storacha/piri/pkg/internal/digestutil"
	"github.com/storacha/piri/pkg/store"
)

type TODO_DsBlobstore struct {
	data datastore.Datastore
}

// Get implements Blobstore.
func (d *TODO_DsBlobstore) Get(ctx context.Context, digest multihash.Multihash, opts ...GetOption) (Object, error) {
	o := &GetOptions{}
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, err
		}
	}

	k := digestutil.Format(digest)
	key := datastore.NewKey(k)
	b, err := d.data.Get(ctx, key)
	if err != nil {
		if errors.Is(err, datastore.ErrNotFound) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}

	if o.ByteRange.Start >= uint64(len(b)) {
		return nil, ErrRangeNotSatisfiable
	}
	if o.ByteRange.End != nil && *o.ByteRange.End >= uint64(len(b)) {
		return nil, ErrRangeNotSatisfiable
	}

	obj := DsObject{bytes: b, byteRange: o.ByteRange}
	return obj, nil
}

func (d *TODO_DsBlobstore) Put(ctx context.Context, digest multihash.Multihash, size uint64, body io.Reader) error {
	b, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("reading body: %w", err)
	}

	if len(b) > int(size) {
		return ErrTooLarge
	}
	if len(b) < int(size) {
		return ErrTooSmall
	}

	k := digestutil.Format(digest)
	key := datastore.NewKey(k)
	err = d.data.Put(ctx, key, b)
	if err != nil {
		return fmt.Errorf("putting blob: %w", err)
	}

	return nil
}

func (d *TODO_DsBlobstore) FileSystem() http.FileSystem {
	return &dsDir{d.data}
}

// NewDsBlobstore creates an [Blobstore] backed by an IPFS datastore.
func NewTODO_DsBlobstore(ds datastore.Datastore) *TODO_DsBlobstore {
	return &TODO_DsBlobstore{ds}
}

var _ Blobstore = (*TODO_DsBlobstore)(nil)
