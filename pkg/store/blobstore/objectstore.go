package blobstore

import (
	"context"
	"io"

	"github.com/multiformats/go-multihash"

	"github.com/storacha/piri/pkg/store/objectstore"
)

type ObjectBlobstore struct {
	data objectstore.Store
}

func (d *ObjectBlobstore) Get(ctx context.Context, digest multihash.Multihash, opts ...GetOption) (Object, error) {
	o := &GetOptions{}
	for _, opt := range opts {
		opt(o)
	}
	return d.data.Get(ctx, digest.HexString(), objectstore.WithRange(objectstore.Range(o.ByteRange)))
}

func (d *ObjectBlobstore) Put(ctx context.Context, digest multihash.Multihash, size uint64, body io.Reader) error {
	return d.data.Put(ctx, digest.HexString(), size, body)
}

// NewObjectBlobstore creates an [Blobstore] backed by an object store.
func NewObjectBlobstore(store objectstore.Store) *ObjectBlobstore {
	return &ObjectBlobstore{store}
}

var _ Blobstore = (*ObjectBlobstore)(nil)
