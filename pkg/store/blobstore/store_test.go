package blobstore

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/stretchr/testify/require"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	piritestutil "github.com/storacha/piri/pkg/internal/testutil"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/objectstore/flatfs"
	minio_store "github.com/storacha/piri/pkg/store/objectstore/minio"
)

func TestBlobstore(t *testing.T) {
	rootdir := path.Join(os.TempDir(), fmt.Sprintf("blobstore%d", time.Now().UnixMilli()))
	t.Cleanup(func() { os.RemoveAll(rootdir) })

	flatfsDir := path.Join(rootdir, "flatfs")
	err := os.MkdirAll(flatfsDir, 0755)
	require.NoError(t, err)

	impls := map[string]Blobstore{
		"memory": NewDatastoreStore(sync.MutexWrap(datastore.NewMapDatastore())),
		"flatfs": NewFlatfsStore(testutil.Must(flatfs.New(flatfsDir, flatfs.NextToLast(2), false))(t)),
	}

	if piritestutil.IsDockerAvailable(t) {
		endpoint := piritestutil.StartMinioContainer(t)
		bucket := uuid.NewString()
		s3store, err := minio_store.New(endpoint, bucket, minio.Options{
			Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
			Secure: false,
		})
		require.NoError(t, err)
		impls["s3"] = NewS3Store(s3store)
	} else if piritestutil.IsRunningInCI(t) && runtime.GOOS == "linux" {
		// This test expects docker to be running in linux CI environments and fails if it's not
		t.Fatalf("docker is expected in CI linux testing environments, but wasn't found")
	}

	for k, s := range impls {
		t.Run(k, func(t *testing.T) {
			t.Run("roundtrip", func(t *testing.T) {
				data := testutil.RandomBytes(t, 10)
				digest := testutil.Must(multihash.Sum(data, multihash.SHA2_256, -1))(t)

				err := s.Put(t.Context(), digest, uint64(len(data)), bytes.NewBuffer(data))
				require.NoError(t, err)

				obj, err := s.Get(t.Context(), digest)
				require.NoError(t, err)
				require.Equal(t, obj.Size(), int64(len(data)))
				require.Equal(t, data, testutil.Must(io.ReadAll(obj.Body()))(t))
			})

			t.Run("not found", func(t *testing.T) {
				data := testutil.RandomBytes(t, 10)
				digest := testutil.Must(multihash.Sum(data, multihash.SHA2_256, -1))(t)

				obj, err := s.Get(t.Context(), digest)
				require.Error(t, err)
				require.Equal(t, store.ErrNotFound, err)
				require.Nil(t, obj)
			})

			t.Run("delete", func(t *testing.T) {
				data := testutil.RandomBytes(t, 10)
				digest := testutil.Must(multihash.Sum(data, multihash.SHA2_256, -1))(t)

				err := s.Put(t.Context(), digest, uint64(len(data)), bytes.NewBuffer(data))
				require.NoError(t, err)

				err = s.Delete(t.Context(), digest)
				require.NoError(t, err)

				_, err = s.Get(t.Context(), digest)
				require.Equal(t, store.ErrNotFound, err)
			})

			t.Run("range not satisfiable", func(t *testing.T) {
				data := testutil.RandomBytes(t, 10)
				digest := testutil.Must(multihash.Sum(data, multihash.SHA2_256, -1))(t)

				err := s.Put(t.Context(), digest, uint64(len(data)), bytes.NewReader(data))
				require.NoError(t, err)

				end := uint64(15)
				_, err = s.Get(t.Context(), digest, WithRange(5, &end))
				require.ErrorIs(t, err, NewRangeNotSatisfiableError(Range{Start: 5, End: &end}))
			})
		})
	}
}
