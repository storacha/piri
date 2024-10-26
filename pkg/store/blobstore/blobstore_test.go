package blobstore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"testing"
	"time"

	"github.com/multiformats/go-multihash"
	"github.com/storacha/storage/pkg/internal/digestutil"
	"github.com/storacha/storage/pkg/internal/testutil"
	"github.com/storacha/storage/pkg/store"
	"github.com/stretchr/testify/require"
)

func TestBlobstore(t *testing.T) {
	rootdir := path.Join(os.TempDir(), fmt.Sprintf("blobstore%d", time.Now().UnixMilli()))
	t.Cleanup(func() { os.RemoveAll(rootdir) })
	tmpdir := path.Join(os.TempDir(), fmt.Sprintf("blobstore-tmp%d", time.Now().UnixMilli()))
	t.Cleanup(func() { os.RemoveAll(tmpdir) })

	impls := map[string]Blobstore{
		"MapBlobstore": NewMapBlobstore(),
		"FsBlobstore":  testutil.Must(NewFsBlobstore(rootdir, tmpdir))(t),
	}

	for k, s := range impls {
		t.Run("roundtrip "+k, func(t *testing.T) {
			data := testutil.RandomBytes(10)
			digest := testutil.Must(multihash.Sum(data, multihash.SHA2_256, -1))(t)

			err := s.Put(context.Background(), digest, uint64(len(data)), bytes.NewBuffer(data))
			require.NoError(t, err)

			obj, err := s.Get(context.Background(), digest)
			require.NoError(t, err)
			require.Equal(t, obj.Size(), int64(len(data)))
			require.Equal(t, data, testutil.Must(io.ReadAll(obj.Body()))(t))
		})

		t.Run("not found "+k, func(t *testing.T) {
			data := testutil.RandomBytes(10)
			digest := testutil.Must(multihash.Sum(data, multihash.SHA2_256, -1))(t)

			obj, err := s.Get(context.Background(), digest)
			require.Error(t, err)
			require.Equal(t, store.ErrNotFound, err)
			require.Nil(t, obj)
		})

		t.Run("data consistency "+k, func(t *testing.T) {
			data := testutil.RandomBytes(10)
			baddata := testutil.RandomBytes(10)
			digest := testutil.Must(multihash.Sum(data, multihash.SHA2_256, -1))(t)

			err := s.Put(context.Background(), digest, uint64(len(data)), bytes.NewBuffer(baddata))
			require.Equal(t, ErrDataInconsistent, err)
		})

		t.Run("filesystemer "+k, func(t *testing.T) {
			data := testutil.RandomBytes(10)
			digest := testutil.Must(multihash.Sum(data, multihash.SHA2_256, -1))(t)

			err := s.Put(context.Background(), digest, uint64(len(data)), bytes.NewBuffer(data))
			require.NoError(t, err)

			fsr, ok := s.(FileSystemer)
			require.True(t, ok)

			f, err := fsr.FileSystem().Open(fmt.Sprintf("/%s", digestutil.Format(digest)))
			require.NoError(t, err)

			b, err := io.ReadAll(f)
			require.NoError(t, err)

			require.Equal(t, data, b)
		})
	}
}
