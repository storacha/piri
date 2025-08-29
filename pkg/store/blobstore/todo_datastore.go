package blobstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/multiformats/go-multihash"

	"github.com/storacha/piri/pkg/internal/digestutil"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/telemetry"
)

type TODO_DsBlobstore struct {
	data          datastore.Datastore
	storeTypeName string
}

// Get implements Blobstore.
func (d *TODO_DsBlobstore) Get(ctx context.Context, digest multihash.Multihash, opts ...GetOption) (Object, error) {
	start := time.Now()
	status := "success"
	defer func() {
		telemetry.RecordStorageExecution(ctx, "get", status, time.Since(start))
	}()

	o := &options{}
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

	obj := DsObject{bytes: b, byteRange: o.byteRange}
	return obj, nil
}

func (d *TODO_DsBlobstore) Put(ctx context.Context, digest multihash.Multihash, size uint64, body io.Reader) error {
	start := time.Now()
	status := "success"
	defer func() {
		telemetry.RecordStorageExecution(ctx, "put", status, time.Since(start))
	}()

	b, err := io.ReadAll(body)
	if err != nil {
		status = "error"
		return fmt.Errorf("reading body: %w", err)
	}

	if len(b) > int(size) {
		status = "error"
		return ErrTooLarge
	}
	if len(b) < int(size) {
		status = "error"
		return ErrTooSmall
	}

	k := digestutil.Format(digest)
	key := datastore.NewKey(k)
	err = d.data.Put(ctx, key, b)
	if err != nil {
		status = "error"
		return fmt.Errorf("putting blob: %w", err)
	}

	// record count of pieces stored
	telemetry.RecordPiecesStored(ctx, d.storeTypeName, 1)

	// record usage (bytes written)
	telemetry.RecordStorageUsage(ctx, d.storeTypeName, int64(len(b)))

	return nil
}

func (d *TODO_DsBlobstore) FileSystem() http.FileSystem {
	return &dsDir{d.data}
}

// NewDsBlobstore creates an [Blobstore] backed by an IPFS datastore.
func NewTODO_DsBlobstore(ds datastore.Datastore) *TODO_DsBlobstore {
	return &TODO_DsBlobstore{
		data:          ds,
		storeTypeName: "todo",
	}
}

var _ Blobstore = (*TODO_DsBlobstore)(nil)
