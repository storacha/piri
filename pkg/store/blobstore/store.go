package blobstore

import (
	"context"
	"errors"
	"io"

	"github.com/ipfs/go-datastore"
	"github.com/multiformats/go-multihash"

	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/objectstore"
	"github.com/storacha/piri/pkg/store/objectstore/dsadapter"
	"github.com/storacha/piri/pkg/store/objectstore/flatfs"
	minio_store "github.com/storacha/piri/pkg/store/objectstore/minio"
)

var _ Blobstore = (*Store)(nil)

// Store wraps an objectstore.Store with a KeyEncoder for S3/MinIO/flatfs backends.
type Store struct {
	backend objectstore.Store
	encoder KeyEncoder
}

// NewS3Store creates a Blobstore backed by an S3/MinIO object store.
func NewS3Store(backend *minio_store.Store) *Store {
	return &Store{
		backend: backend,
		encoder: Base32KeyEncoder{},
	}
}

// NewFlatfsStore creates a Blobstore backed by a flatfs object store.
func NewFlatfsStore(backend *flatfs.Store) *Store {
	return &Store{
		backend: backend,
		encoder: Base32KeyEncoder{},
	}
}

// NewDatastoreStore creates a Blobstore backed by a datastore.Datastore.
// Useful for testing with sync.MutexWrap(datastore.NewMapDatastore()).
func NewDatastoreStore(ds datastore.Datastore) *Store {
	return &Store{
		backend: dsadapter.New(ds),
		encoder: PlainKeyEncoder{},
	}
}

func (s *Store) Get(ctx context.Context, digest multihash.Multihash, opts ...GetOption) (Object, error) {
	o := &GetOptions{}
	for _, opt := range opts {
		opt(o)
	}
	obj, err := s.backend.Get(ctx, s.encoder.EncodeKey(digest), objectstore.WithRange(objectstore.Range(o.ByteRange)))
	if err != nil {
		if errors.Is(err, objectstore.ErrNotExist) {
			return nil, store.ErrNotFound
		}
		var erns objectstore.ErrRangeNotSatisfiable
		if errors.As(err, &erns) {
			return nil, NewRangeNotSatisfiableError(Range{Start: erns.Range.Start, End: erns.Range.End})
		}
		return nil, err
	}
	return obj, nil
}

func (s *Store) Put(ctx context.Context, digest multihash.Multihash, size uint64, body io.Reader) error {
	return s.backend.Put(ctx, s.encoder.EncodeKey(digest), size, body)
}

func (s *Store) Delete(ctx context.Context, digest multihash.Multihash) error {
	return s.backend.Delete(ctx, s.encoder.EncodeKey(digest))
}
