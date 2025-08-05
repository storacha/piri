package blobs

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/multiformats/go-multihash"
	ed25519 "github.com/storacha/go-ucanto/principal/ed25519/signer"
	"github.com/stretchr/testify/require"

	"github.com/storacha/go-libstoracha/testutil"

	"github.com/storacha/piri/pkg/fx/echo"
	"github.com/storacha/piri/pkg/internal/digestutil"
	"github.com/storacha/piri/pkg/presigner"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/allocationstore/allocation"
	"github.com/storacha/piri/pkg/store/blobstore"
)

func TestServer(t *testing.T) {
	mux := echo.NewEcho()
	httpsrv := httptest.NewServer(mux)
	t.Cleanup(httpsrv.Close)

	srvurl, err := url.Parse(httpsrv.URL)
	require.NoError(t, err)

	rootdir := path.Join(os.TempDir(), fmt.Sprintf("blobstore%d", time.Now().UnixMilli()))
	t.Cleanup(func() { os.RemoveAll(rootdir) })
	tmpdir := path.Join(os.TempDir(), fmt.Sprintf("blobstore-tmp%d", time.Now().UnixMilli()))
	t.Cleanup(func() { os.RemoveAll(tmpdir) })

	blobs, err := blobstore.NewFsBlobstore(rootdir, tmpdir)
	require.NoError(t, err)

	signer := testutil.RandomSigner(t)
	accessKeyID := signer.DID().String()
	secretAccessKey := testutil.Must(ed25519.Format(signer))(t)
	presigner, err := presigner.NewS3RequestPresigner(accessKeyID, secretAccessKey, *srvurl, "blob")
	require.NoError(t, err)

	allocs, err := allocationstore.NewDsAllocationStore(datastore.NewMapDatastore())
	require.NoError(t, err)

	srv, err := NewServer(presigner, allocs, blobs)
	require.NoError(t, err)

	srv.RegisterRoutes(mux)

	t.Run("get blob", func(t *testing.T) {
		data := testutil.RandomBytes(t, 32)
		digest, err := multihash.Sum(data, multihash.SHA2_256, -1)
		require.NoError(t, err)

		err = blobs.Put(t.Context(), digest, uint64(len(data)), bytes.NewReader(data))
		require.NoError(t, err)

		requireRetrievableBlob(t, *srvurl, digest, data)
	})

	t.Run("put blob", func(t *testing.T) {
		t.Run("basic", func(t *testing.T) {
			data := testutil.RandomBytes(t, 32)
			digest, err := multihash.Sum(data, multihash.SHA2_256, -1)
			require.NoError(t, err)

			// create a fake allocation
			err = allocs.Put(t.Context(), randomAllocation(t, digest, uint64(len(data))))
			require.NoError(t, err)

			putBlob(t, presigner, digest, data, http.StatusOK)
			requireRetrievableBlob(t, *srvurl, digest, data)
		})

		t.Run("allow repeated write", func(t *testing.T) {
			data := testutil.RandomBytes(t, 32)
			digest, err := multihash.Sum(data, multihash.SHA2_256, -1)
			require.NoError(t, err)

			// create fake allocation
			err = allocs.Put(t.Context(), randomAllocation(t, digest, uint64(len(data))))
			require.NoError(t, err)

			putBlob(t, presigner, digest, data, http.StatusOK)
			putBlob(t, presigner, digest, data, http.StatusOK)
			requireRetrievableBlob(t, *srvurl, digest, data)
		})

		t.Run("persist previous blob on repeated write failure", func(t *testing.T) {
			data := testutil.RandomBytes(t, 32)
			digest, err := multihash.Sum(data, multihash.SHA2_256, -1)
			require.NoError(t, err)

			// create a fake allocation
			err = allocs.Put(t.Context(), randomAllocation(t, digest, uint64(len(data))))
			require.NoError(t, err)

			putBlob(t, presigner, digest, data, http.StatusOK)
			requireRetrievableBlob(t, *srvurl, digest, data)

			putBlob(t, presigner, digest, []byte{1}, http.StatusConflict)
			requireRetrievableBlob(t, *srvurl, digest, data)
		})
	})
}

func randomAllocation(t *testing.T, digest multihash.Multihash, size uint64) allocation.Allocation {
	return allocation.Allocation{
		Space: testutil.RandomDID(t),
		Blob: allocation.Blob{
			Digest: digest,
			Size:   size,
		},
		Expires: uint64(time.Now().Unix() + 900),
		Cause:   testutil.RandomCID(t),
	}
}

func putBlob(t *testing.T, presigner presigner.RequestPresigner, digest multihash.Multihash, data []byte, expectStatus int) {
	url, hd, err := presigner.SignUploadURL(t.Context(), digest, uint64(len(data)), 900)
	require.NoError(t, err)

	req, err := http.NewRequest("PUT", url.String(), bytes.NewReader(data))
	require.NoError(t, err)
	req.Header = hd

	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, expectStatus, res.StatusCode)
}

func requireRetrievableBlob(t *testing.T, endpoint url.URL, digest multihash.Multihash, data []byte) {
	bloburl := endpoint
	blobpath, err := url.JoinPath(bloburl.Path, "blob", digestutil.Format(digest))
	require.NoError(t, err)

	bloburl.Path = blobpath

	res, err := http.Get(bloburl.String())
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, res.StatusCode)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, data, body)
}
