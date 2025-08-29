package blobstore

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/multiformats/go-multihash"

	"github.com/storacha/piri/pkg/internal/digestutil"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/telemetry"
)

type TODOMapBlobstore struct {
	data          map[string][]byte
	storeTypeName string
}

func (mb *TODOMapBlobstore) Get(ctx context.Context, digest multihash.Multihash, opts ...GetOption) (Object, error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	k := digestutil.Format(digest)
	b, ok := mb.data[k]
	if !ok {
		return nil, store.ErrNotFound
	}

	obj := MapObject{bytes: b, byteRange: o.byteRange}
	return obj, nil
}

func (mb *TODOMapBlobstore) Put(ctx context.Context, digest multihash.Multihash, size uint64, body io.Reader) error {
	start := time.Now()
	status := "success"
	defer func() {
		telemetry.RecordStorageExecution(ctx, "put", status, time.Since(start))
	}()

	b, err := io.ReadAll(body)
	if err != nil {
		status = "failed"
		return fmt.Errorf("reading body: %w", err)
	}

	if len(b) > int(size) {
		status = "failed"
		return ErrTooLarge
	}
	if len(b) < int(size) {
		status = "failed"
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
	return &TODOMapBlobstore{
		data:          data,
		storeTypeName: "map_blob",
	}
}
