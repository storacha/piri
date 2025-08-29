package blobstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/multiformats/go-multihash"

	"github.com/storacha/piri/pkg/internal/digestutil"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/telemetry"
)

type DsObject = MapObject

type DsBlobstore struct {
	data          datastore.Datastore
	storeTypeName string
}

// Get implements Blobstore.
func (d *DsBlobstore) Get(ctx context.Context, digest multihash.Multihash, opts ...GetOption) (Object, error) {
	// record storage execution time
	start := time.Now()
	status := "success"
	defer func() {
		telemetry.RecordStorageExecution(ctx, "get", status, time.Since(start))
	}()

	o := &options{}
	for _, opt := range opts {
		if err := opt(o); err != nil {
			status = "failed"
			return nil, err
		}
	}

	k := digestutil.Format(digest)
	b, err := d.data.Get(ctx, datastore.NewKey(k))
	if err != nil {
		if errors.Is(err, datastore.ErrNotFound) {
			status = "not_found"
			return nil, store.ErrNotFound
		}

		status = "failed"
		return nil, err
	}

	obj := DsObject{bytes: b, byteRange: o.byteRange}
	return obj, nil
}

func (d *DsBlobstore) Put(ctx context.Context, digest multihash.Multihash, size uint64, body io.Reader) error {
	start := time.Now()
	status := "success"
	defer func() {
		telemetry.RecordStorageExecution(ctx, "put", status, time.Since(start))
	}()

	info, err := multihash.Decode(digest)
	if err != nil {
		status = "failed"
		return fmt.Errorf("decoding digest: %w", err)
	}
	if info.Code != multihash.SHA2_256 {
		return fmt.Errorf("unsupported digest: 0x%x", info.Code)
	}

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

	hash := sha256.New()
	hash.Write(b)

	if !bytes.Equal(hash.Sum(nil), info.Digest) {
		status = "failed"
		return ErrDataInconsistent
	}

	k := digestutil.Format(digest)
	err = d.data.Put(ctx, datastore.NewKey(k), b)
	if err != nil {
		return fmt.Errorf("putting blob: %w", err)
	}

	// record count of pieces stored
	telemetry.RecordPiecesStored(ctx, d.storeTypeName, 1)

	// record usage (bytes written)
	telemetry.RecordStorageUsage(ctx, d.storeTypeName, int64(len(b)))

	return nil
}

func (d *DsBlobstore) FileSystem() http.FileSystem {
	return &dsDir{d.data}
}

// NewDsBlobstore creates an [Blobstore] backed by an IPFS datastore.
func NewDsBlobstore(ds datastore.Datastore) *DsBlobstore {
	return &DsBlobstore{
		data:          ds,
		storeTypeName: "blob",
	}
}

var _ Blobstore = (*DsBlobstore)(nil)

type dsDir struct {
	data datastore.Datastore
}

var _ http.FileSystem = (*mapDir)(nil)

func (d *dsDir) Open(path string) (http.File, error) {
	name := path[1:]

	data, err := d.data.Get(context.Background(), datastore.NewKey(name))
	if err != nil {
		if errors.Is(err, datastore.ErrNotFound) {
			return nil, fs.ErrNotExist
		}
		return nil, err
	}

	return &dsFile{
		Reader: bytes.NewReader(data),
		info:   dsFileInfo{name, int64(len(data))},
	}, nil
}

type dsFile struct {
	*bytes.Reader
	info fs.FileInfo
}

func (d *dsFile) Close() error {
	return nil
}

func (d *dsFile) Readdir(count int) ([]fs.FileInfo, error) {
	panic("unimplemented") // should not be called - there are no directories
}

func (d *dsFile) Stat() (fs.FileInfo, error) {
	return d.info, nil
}

var _ http.File = (*mapFile)(nil)

type dsFileInfo = mapFileInfo
