package blobstore

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/multiformats/go-multihash"

	"github.com/storacha/piri/pkg/internal/digestutil"
	"github.com/storacha/piri/pkg/store"
)

type TODOMapBlobstore struct {
	data map[string][]byte
}

func (mb *TODOMapBlobstore) Get(ctx context.Context, digest multihash.Multihash, opts ...GetOption) (Object, error) {
	o := &GetOptions{}
	for _, opt := range opts {
		opt(o)
	}

	k := digestutil.Format(digest)
	b, ok := mb.data[k]
	if !ok {
		return nil, store.ErrNotFound
	}

	if o.ByteRange.Start >= uint64(len(b)) {
		return nil, ErrRangeNotSatisfiable
	}
	if o.ByteRange.End != nil && *o.ByteRange.End >= uint64(len(b)) {
		return nil, ErrRangeNotSatisfiable
	}

	obj := MapObject{bytes: b, byteRange: o.ByteRange}
	return obj, nil
}

func (mb *TODOMapBlobstore) Put(ctx context.Context, digest multihash.Multihash, size uint64, body io.Reader) error {
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
	mb.data[k] = b

	return nil
}

func (mb *TODOMapBlobstore) FileSystem() http.FileSystem {
	return &mapDir{mb.data}
}

var _ Blobstore = (*TODOMapBlobstore)(nil)

func NewTODOMapBlobstore() *TODOMapBlobstore {
	data := map[string][]byte{}
	return &TODOMapBlobstore{data}
}
