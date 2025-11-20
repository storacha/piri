package retrieval_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/storacha/go-libstoracha/capabilities/space/content"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/go-ucanto/client"
	retrievalclient "github.com/storacha/go-ucanto/client/retrieval"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/core/result"
	ucanhttp "github.com/storacha/go-ucanto/transport/http"
	"github.com/storacha/go-ucanto/ucan"
	piritutil "github.com/storacha/piri/pkg/internal/testutil"
	"github.com/stretchr/testify/require"

	"github.com/storacha/piri/pkg/server"
	"github.com/storacha/piri/pkg/service/retrieval"
	"github.com/storacha/piri/pkg/service/storage"
	"github.com/storacha/piri/pkg/store/allocationstore/allocation"
)

func TestSpaceContentRetrieve(t *testing.T) {
	ctx := t.Context()
	uploadServiceConn := testutil.Must(client.NewConnection(testutil.Service.DID(), ucanhttp.NewChannel(testutil.TestURL)))(t)
	storageSvc, err := storage.New(uploadServiceConn, storage.WithIdentity(testutil.Alice), storage.WithLogLevel("*", "warn"))
	require.NoError(t, err)
	err = storageSvc.Startup(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		storageSvc.Close(ctx)
	})

	retrievalSvc := retrieval.New(testutil.Alice, storageSvc.Blobs().Store(), storageSvc.Blobs().Allocations())

	port := piritutil.GetFreePort(t)
	srvMux, err := server.NewServer(storageSvc, retrievalSvc)
	require.NoError(t, err)
	srv := &http.Server{
		Addr:    fmt.Sprintf("localhost:%d", port),
		Handler: srvMux,
	}
	go func() {
		err = srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			require.NoError(t, err)
		}
	}()
	t.Cleanup(func() {
		srv.Close()
	})
	publicURL := testutil.Must(url.Parse(fmt.Sprintf("http://localhost:%d", port)))(t)

	t.Run("space/content/retrieve", func(t *testing.T) {
		space := testutil.RandomSigner(t)
		randBytes := testutil.RandomBytes(t, 256)
		blob := struct {
			bytes []byte
			cid   cid.Cid
		}{randBytes, cid.NewCidV1(cid.Raw, testutil.MultihashFromBytes(t, randBytes))}

		storageSvc.Blobs().Allocations().Put(t.Context(), allocation.Allocation{
			Blob: allocation.Blob{
				Digest: blob.cid.Hash(),
				Size:   uint64(len(blob.bytes)),
			},
			Space:   space.DID(),
			Expires: 0,
			Cause:   testutil.RandomCID(t),
		})
		err := storageSvc.Blobs().Store().Put(
			t.Context(),
			blob.cid.Hash(),
			uint64(len(blob.bytes)),
			bytes.NewReader(blob.bytes),
		)
		require.NoError(t, err)

		prf := delegation.FromDelegation(
			testutil.Must(
				delegation.Delegate(
					space,
					testutil.Bob,
					[]ucan.Capability[content.RetrieveCaveats]{
						ucan.NewCapability(
							content.RetrieveAbility,
							space.DID().String(),
							content.RetrieveCaveats{
								Blob:  content.BlobDigest{Digest: blob.cid.Hash()},
								Range: content.Range{Start: 0, End: uint64(len(blob.bytes) - 1)},
							},
						),
					},
				),
			)(t),
		)

		url := publicURL.JoinPath("piece", blob.cid.String())
		conn, err := retrievalclient.NewConnection(testutil.Alice, url)
		require.NoError(t, err)

		inv, err := invocation.Invoke(
			testutil.Bob,
			testutil.Alice,
			content.Retrieve.New(
				space.DID().String(),
				content.RetrieveCaveats{
					Blob:  content.BlobDigest{Digest: blob.cid.Hash()},
					Range: content.Range{Start: 0, End: 1},
				},
			),
			delegation.WithProof(prf),
		)
		require.NoError(t, err)

		xres, hres, err := retrievalclient.Execute(t.Context(), inv, conn)
		require.NoError(t, err)

		expectStatus := http.StatusPartialContent
		expectHeaders := http.Header{
			http.CanonicalHeaderKey("Content-Length"): []string{fmt.Sprintf("%d", 2)},
			http.CanonicalHeaderKey("Content-Range"):  []string{fmt.Sprintf("bytes %d-%d/%d", 0, 1, len(blob.bytes))},
		}
		expectBody := blob.bytes[0:2]

		require.Equal(t, expectStatus, hres.Status())
		for k, v := range expectHeaders {
			require.Equal(t, v, hres.Headers().Values(k))
		}
		require.Equal(t, expectBody, testutil.Must(io.ReadAll(hres.Body()))(t))

		rcptLink, ok := xres.Get(inv.Link())
		require.True(t, ok)

		rcpt, err := receipt.NewAnyReceiptReader().Read(rcptLink, xres.Blocks())
		require.NoError(t, err)

		_, x := result.Unwrap(rcpt.Out())
		require.Nil(t, x)
	})
}
