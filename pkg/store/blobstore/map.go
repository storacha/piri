package blobstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"time"

	"github.com/multiformats/go-multihash"
	"github.com/storacha/piri/pkg/internal/digestutil"
	"github.com/storacha/piri/pkg/store"
)

type MapObject struct {
	bytes     []byte
	byteRange Range
}

func (o MapObject) Size() int64 {
	return int64(len(o.bytes))
}

func (o MapObject) Body() io.ReadCloser {
	b := o.bytes
	if o.byteRange.Offset > 0 {
		b = b[o.byteRange.Offset:]
	}
	if o.byteRange.Length != nil {
		b = b[0:*o.byteRange.Length]
	}
	return io.NopCloser(bytes.NewReader(b))
}

type MapBlobstore struct {
	data map[string][]byte
}

func (mb *MapBlobstore) Get(ctx context.Context, digest multihash.Multihash, opts ...GetOption) (Object, error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	k := digestutil.Format(digest)
	b, ok := mb.data[k]
	if !ok {
		return nil, store.ErrNotFound
	}

	if o.byteRange.Offset >= uint64(len(b)) {
		return nil, ErrRangeNotSatisfiable
	}
	if o.byteRange.Length != nil && o.byteRange.Offset+*o.byteRange.Length > uint64(len(b)) {
		return nil, ErrRangeNotSatisfiable
	}

	obj := MapObject{bytes: b, byteRange: o.byteRange}
	return obj, nil
}

func (mb *MapBlobstore) Put(ctx context.Context, digest multihash.Multihash, size uint64, body io.Reader) error {
	info, err := multihash.Decode(digest)
	if err != nil {
		return fmt.Errorf("decoding digest: %w", err)
	}
	if info.Code != multihash.SHA2_256 {
		return fmt.Errorf("unsupported digest: 0x%x", info.Code)
	}

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

	hash := sha256.New()
	hash.Write(b)

	if !bytes.Equal(hash.Sum(nil), info.Digest) {
		return ErrDataInconsistent
	}

	k := digestutil.Format(digest)
	mb.data[k] = b

	return nil
}

func (mb *MapBlobstore) FileSystem() http.FileSystem {
	return &mapDir{mb.data}
}

var _ Blobstore = (*MapBlobstore)(nil)

// NewMapBlobstore creates a [Blobstore] backed by an in-memory map.
func NewMapBlobstore() *MapBlobstore {
	data := map[string][]byte{}
	return &MapBlobstore{data}
}

type mapDir struct {
	data map[string][]byte
}

var _ http.FileSystem = (*mapDir)(nil)

func (d *mapDir) Open(path string) (http.File, error) {
	name := path[1:]
	data, ok := d.data[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return &mapFile{
		Reader: bytes.NewReader(data),
		info:   mapFileInfo{name, int64(len(data))},
	}, nil
}

type mapFile struct {
	*bytes.Reader
	info fs.FileInfo
}

func (m *mapFile) Close() error {
	return nil
}

func (m *mapFile) Readdir(count int) ([]fs.FileInfo, error) {
	panic("unimplemented") // should not be called - there are no directories
}

func (m *mapFile) Stat() (fs.FileInfo, error) {
	return m.info, nil
}

var _ http.File = (*mapFile)(nil)

type mapFileInfo struct {
	name string
	size int64
}

func (mfi mapFileInfo) Name() string       { return mfi.name }
func (mfi mapFileInfo) Size() int64        { return mfi.size }
func (mfi mapFileInfo) Mode() os.FileMode  { return 0444 }
func (mfi mapFileInfo) ModTime() time.Time { return time.Time{} }
func (mfi mapFileInfo) IsDir() bool        { return false }
func (mfi mapFileInfo) Sys() interface{}   { return nil }

var _ fs.FileInfo = (*mapFileInfo)(nil)
