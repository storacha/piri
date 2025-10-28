package blobstore

import (
	"context"
	"io"

	"github.com/multiformats/go-base32"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/piri/pkg/store/objectstore"
)

type ObjectBlobstore struct {
	data objectstore.Store
}

// NewObjectBlobstore creates an [Blobstore] backed by an object store.
func NewObjectBlobstore(store objectstore.Store) *ObjectBlobstore {
	return &ObjectBlobstore{store}
}

var _ Blobstore = (*ObjectBlobstore)(nil)

func (d *ObjectBlobstore) Get(ctx context.Context, digest multihash.Multihash, opts ...GetOption) (Object, error) {
	o := &GetOptions{}
	for _, opt := range opts {
		opt(o)
	}
	return d.data.Get(ctx, encodeKey(digest), objectstore.WithRange(objectstore.Range(o.ByteRange)))
}

func (d *ObjectBlobstore) Put(ctx context.Context, digest multihash.Multihash, size uint64, body io.Reader) error {
	return d.data.Put(ctx, encodeKey(digest), size, body)
}

// Adapted from
// https://github.com/ipfs/boxo/blob/8c17f11f399062878a8093f12cedce56877dbb6f/datastore/dshelp/key.go#L13-L18
func encodeKey(rawKey []byte) string {
	buf := make([]byte, base32.RawStdEncoding.EncodedLen(len(rawKey)))
	base32.RawStdEncoding.Encode(buf, rawKey)
	return string(buf)
}
