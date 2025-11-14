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
	"os"
	"path"
	"strings"

	"github.com/multiformats/go-multibase"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/digestutil"
	"github.com/storacha/piri/pkg/store"
)

type FileObject struct {
	name      string
	size      int64
	byteRange Range
}

func (o FileObject) Size() int64 {
	return o.size
}

func (o FileObject) Body() io.ReadCloser {
	r, w := io.Pipe()
	f, err := os.Open(o.name)
	if err != nil {
		r.CloseWithError(err)
		return r
	}

	if o.byteRange.Start > 0 {
		f.Seek(int64(o.byteRange.Start), io.SeekStart)
	}

	go func() {
		var err error
		if o.byteRange.End != nil {
			_, err = io.CopyN(w, f, int64(*o.byteRange.End-o.byteRange.Start+1))
		} else {
			_, err = io.Copy(w, f)
		}
		f.Close()
		w.CloseWithError(err)
	}()

	return r
}

func encodePath(digest multihash.Multihash) string {
	str := digestutil.Format(digest)
	var parts []string
	for i := 0; i < len(str); i += 2 {
		end := i + 2
		if end > len(str) {
			end = len(str)
		}
		parts = append(parts, str[i:end])
	}
	return path.Join(parts...)
}

type FsBlobstore struct {
	rootdir string
	tmpdir  string
}

// FileSystem returns a filesystem interface for reading blobs.
func (b *FsBlobstore) FileSystem() http.FileSystem {
	return &fsDir{http.Dir(b.rootdir)}
}

func (b *FsBlobstore) Get(ctx context.Context, digest multihash.Multihash, opts ...GetOption) (Object, error) {
	o := &GetOptions{}
	for _, opt := range opts {
		opt(o)
	}

	n := path.Join(b.rootdir, encodePath(digest))
	f, err := os.Open(n)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	inf, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	if !rangeSatisfiable(o.ByteRange.Start, o.ByteRange.End, uint64(inf.Size())) {
		return nil, NewRangeNotSatisfiableError(o.ByteRange)
	}

	return FileObject{name: n, size: inf.Size(), byteRange: o.ByteRange}, nil
}

func (b *FsBlobstore) Put(ctx context.Context, digest multihash.Multihash, size uint64, body io.Reader) error {
	info, err := multihash.Decode(digest)
	if err != nil {
		return fmt.Errorf("decoding digest: %w", err)
	}
	if info.Code != multihash.SHA2_256 {
		return fmt.Errorf("unsupported digest: 0x%x", info.Code)
	}

	tmpname := path.Join(b.tmpdir, encodePath(digest))
	err = os.MkdirAll(path.Dir(tmpname), 0755)
	if err != nil {
		return fmt.Errorf("creating intermediate directories: %w", err)
	}

	f, err := os.Create(tmpname)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}

	closed := false
	moved := false
	defer func() {
		if !closed {
			f.Close()
		}
		if !moved {
			os.Remove(tmpname)
		}
	}()

	hash := sha256.New()
	tee := io.TeeReader(body, hash)

	written, err := io.Copy(f, tee)
	if err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	if written > int64(size) {
		return ErrTooLarge
	}
	if written < int64(size) {
		return ErrTooSmall
	}

	if !bytes.Equal(hash.Sum(nil), info.Digest) {
		return ErrDataInconsistent
	}

	name := path.Join(b.rootdir, encodePath(digest))
	err = os.MkdirAll(path.Dir(name), 0755)
	if err != nil {
		return fmt.Errorf("creating intermediate directories: %w", err)
	}

	_ = f.Close()
	closed = true

	err = move(tmpname, name)
	if err != nil {
		return fmt.Errorf("moving file: %w", err)
	}
	moved = true

	return nil
}

func move(source, destination string) error {
	err := os.Rename(source, destination)
	if err != nil && strings.Contains(err.Error(), "invalid cross-device link") {
		return moveCrossDevice(source, destination)
	}
	return err
}

func moveCrossDevice(source, destination string) error {
	src, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("opening source: %w", err)
	}
	dst, err := os.Create(destination)
	if err != nil {
		src.Close()
		return fmt.Errorf("creating destination: %w", err)
	}
	_, err = io.Copy(dst, src)
	src.Close()
	dst.Close()
	if err != nil {
		return fmt.Errorf("copying file: %w", err)
	}
	fi, err := os.Stat(source)
	if err != nil {
		os.Remove(destination)
		return fmt.Errorf("getting file stats: %w", err)
	}
	err = os.Chmod(destination, fi.Mode())
	if err != nil {
		os.Remove(destination)
		return fmt.Errorf("changing file mode: %w", err)
	}
	err = os.Remove(source)
	if err != nil {
		return fmt.Errorf("removing source: %w", err)
	}
	return nil
}

var _ Blobstore = (*FsBlobstore)(nil)
var _ FileSystemer = (*FsBlobstore)(nil)

// NewFsBlobstore creates a [Blobstore] backed by the local filesystem.
// The tmpdir parameter is optional, defaulting to [os.TempDir] + "blobs".
func NewFsBlobstore(rootdir string, tmpdir string) (*FsBlobstore, error) {
	err := os.MkdirAll(rootdir, 0755)
	if err != nil {
		return nil, fmt.Errorf("root directory not writable: %w", err)
	}
	if tmpdir == "" {
		tmpdir = path.Join(os.TempDir(), "blobs")
	}
	if tmpdir == rootdir {
		return nil, errors.New("tmp directory must NOT be the same as root directory")
	}
	err = os.MkdirAll(tmpdir, 0755)
	if err != nil {
		return nil, fmt.Errorf("tmp directory not writable: %w", err)
	}
	return &FsBlobstore{rootdir, tmpdir}, nil
}

type fsDir struct {
	fs http.FileSystem
}

var _ http.FileSystem = (*fsDir)(nil)

func (d *fsDir) Open(path string) (http.File, error) {
	_, bytes, err := multibase.Decode(path[1:])
	if err != nil {
		return nil, fs.ErrNotExist
	}
	digest, err := multihash.Cast(bytes)
	if err != nil {
		return nil, fs.ErrNotExist
	}
	return d.fs.Open(encodePath(digest))
}
