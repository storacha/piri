package blobstore

import (
	"context"
	"errors"
	"io"

	"github.com/multiformats/go-multibase"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/piri/pkg/store"
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
	obj, err := d.data.Get(ctx, encodeKey(digest), objectstore.WithRange(objectstore.Range(o.ByteRange)))
	if err != nil {
		if errors.Is(err, objectstore.ErrNotExist) {
			return nil, store.ErrNotFound
		}
		var erns objectstore.ErrRangeNotSatisfiable
		if errors.As(err, &erns) {
			return nil, ErrRangeNotSatisfiable{Range: Range{Start: erns.Range.Start, End: erns.Range.End}}
		}
		return nil, err
	}
	return obj, nil
}

func (d *ObjectBlobstore) Put(ctx context.Context, digest multihash.Multihash, size uint64, body io.Reader) error {
	return d.data.Put(ctx, encodeKey(digest), size, body)
}

// Adapted from
// https://github.com/ipfs/boxo/blob/8c17f11f399062878a8093f12cedce56877dbb6f/datastore/dshelp/key.go#L13-L18
func encodeKey(rawKey []byte) string {
	b32, _ := multibase.Encode(multibase.Base32, rawKey)
	return b32[1:]
}
