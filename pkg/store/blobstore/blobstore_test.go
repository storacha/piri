package blobstore

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"testing"
	"time"

	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/digestutil"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/objectstore/flatfs"
	"github.com/stretchr/testify/require"
)

func TestBlobstore(t *testing.T) {
	rootdir := path.Join(os.TempDir(), fmt.Sprintf("blobstore%d", time.Now().UnixMilli()))
	t.Cleanup(func() { os.RemoveAll(rootdir) })
	tmpdir := path.Join(os.TempDir(), fmt.Sprintf("blobstore-tmp%d", time.Now().UnixMilli()))
	t.Cleanup(func() { os.RemoveAll(tmpdir) })

	impls := map[string]Blobstore{
		"MapBlobstore":    NewMapBlobstore(),
		"FsBlobstore":     testutil.Must(NewFsBlobstore(path.Join(rootdir, "fs"), tmpdir))(t),
		"ObjectBlobstore": NewObjectBlobstore(testutil.Must(flatfs.New(path.Join(rootdir, "flatfs"), flatfs.NextToLast(2), false))(t)),
	}

	for k, s := range impls {
		t.Run("roundtrip "+k, func(t *testing.T) {
			data := testutil.RandomBytes(t, 10)
			digest := testutil.Must(multihash.Sum(data, multihash.SHA2_256, -1))(t)

			err := s.Put(t.Context(), digest, uint64(len(data)), bytes.NewBuffer(data))
			require.NoError(t, err)

			obj, err := s.Get(t.Context(), digest)
			require.NoError(t, err)
			require.Equal(t, obj.Size(), int64(len(data)))
			require.Equal(t, data, testutil.Must(io.ReadAll(obj.Body()))(t))
		})

		t.Run("not found "+k, func(t *testing.T) {
			data := testutil.RandomBytes(t, 10)
			digest := testutil.Must(multihash.Sum(data, multihash.SHA2_256, -1))(t)

			obj, err := s.Get(t.Context(), digest)
			require.Error(t, err)
			require.Equal(t, store.ErrNotFound, err)
			require.Nil(t, obj)
		})

		t.Run("data consistency "+k, func(t *testing.T) {
			if k == "ObjectBlobstore" {
				t.SkipNow() // Object blobstore does not (currently) check data consistency on write
			}
			data := testutil.RandomBytes(t, 10)
			baddata := testutil.RandomBytes(t, 10)
			digest := testutil.Must(multihash.Sum(data, multihash.SHA2_256, -1))(t)

			err := s.Put(t.Context(), digest, uint64(len(data)), bytes.NewBuffer(baddata))
			require.Equal(t, ErrDataInconsistent, err)
		})

		t.Run("filesystemer "+k, func(t *testing.T) {
			if k == "ObjectBlobstore" {
				t.SkipNow() // Object blobstore does not support filesystemer
			}
			data := testutil.RandomBytes(t, 10)
			digest := testutil.Must(multihash.Sum(data, multihash.SHA2_256, -1))(t)

			err := s.Put(t.Context(), digest, uint64(len(data)), bytes.NewBuffer(data))
			require.NoError(t, err)

			fsr, ok := s.(FileSystemer)
			require.True(t, ok)

			f, err := fsr.FileSystem().Open(fmt.Sprintf("/%s", digestutil.Format(digest)))
			require.NoError(t, err)

			b, err := io.ReadAll(f)
			require.NoError(t, err)

			require.Equal(t, data, b)
		})

		t.Run("range not satisfiable "+k, func(t *testing.T) {
			data := testutil.RandomBytes(t, 10)
			digest := testutil.Must(multihash.Sum(data, multihash.SHA2_256, -1))(t)

			err := s.Put(t.Context(), digest, uint64(len(data)), bytes.NewReader(data))
			require.NoError(t, err)

			end := uint64(15)
			_, err = s.Get(t.Context(), digest, WithRange(5, &end))
			require.ErrorIs(t, err, ErrRangeNotSatisfiable{Range: Range{Start: 5, End: &end}})
		})
	}
}
