package retrieval_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/ipfs/go-cid"
	blobcaps "github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-libstoracha/capabilities/space/content"
	"github.com/storacha/go-libstoracha/testutil"
	retrievalclient "github.com/storacha/go-ucanto/client/retrieval"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/storacha/piri/pkg/fx/app"
	piritestutil "github.com/storacha/piri/pkg/internal/testutil"
	"github.com/storacha/piri/pkg/service/storage"
	"github.com/storacha/piri/pkg/store/allocationstore/allocation"
)

func TestFXSpaceContentRetrieve(t *testing.T) {
	var svc storage.Service

	appConfig := piritestutil.NewTestConfig(t, piritestutil.WithSigner(testutil.Alice))
	testApp := fxtest.New(t,
		app.CommonModules(appConfig),
		app.UCANModule,
		fx.Populate(&svc),
	)

	testApp.RequireStart()
	defer testApp.RequireStop()

	t.Run("space/content/retrieve", func(t *testing.T) {
		space := testutil.RandomSigner(t)
		randBytes := testutil.RandomBytes(t, 256)
		blob := struct {
			bytes []byte
			cid   cid.Cid
		}{randBytes, cid.NewCidV1(cid.Raw, testutil.MultihashFromBytes(t, randBytes))}

		svc.Blobs().Allocations().Put(t.Context(), allocation.Allocation{
			Blob: allocation.Blob{
				Digest: blob.cid.Hash(),
				Size:   uint64(len(blob.bytes)),
			},
			Space:   space.DID(),
			Expires: 0,
			Cause:   testutil.RandomCID(t),
		})
		err := svc.Blobs().Store().Put(
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

		url := appConfig.Server.PublicURL.JoinPath("piece", blob.cid.String())
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

func TestFXBlobRetrieve(t *testing.T) {
	var svc storage.Service

	appConfig := piritestutil.NewTestConfig(t, piritestutil.WithSigner(testutil.Alice))
	testApp := fxtest.New(t,
		app.CommonModules(appConfig),
		app.UCANModule,
		fx.Populate(&svc),
	)

	testApp.RequireStart()
	defer testApp.RequireStop()

	t.Run("blob/retrieve", func(t *testing.T) {
		randBytes := testutil.RandomBytes(t, 256)
		blob := struct {
			bytes []byte
			cid   cid.Cid
		}{randBytes, cid.NewCidV1(cid.Raw, testutil.MultihashFromBytes(t, randBytes))}

		err := svc.Blobs().Store().Put(
			t.Context(),
			blob.cid.Hash(),
			uint64(len(blob.bytes)),
			bytes.NewReader(blob.bytes),
		)
		require.NoError(t, err)

		prf := delegation.FromDelegation(
			testutil.Must(
				delegation.Delegate(
					testutil.Alice,
					testutil.Bob,
					[]ucan.Capability[blobcaps.RetrieveCaveats]{
						ucan.NewCapability(
							blobcaps.RetrieveAbility,
							testutil.Alice.DID().String(),
							blobcaps.RetrieveCaveats{
								Blob: blobcaps.Blob{Digest: blob.cid.Hash()},
							},
						),
					},
				),
			)(t),
		)

		url := appConfig.Server.PublicURL.JoinPath("piece", blob.cid.String())
		conn, err := retrievalclient.NewConnection(testutil.Alice, url)
		require.NoError(t, err)

		inv, err := invocation.Invoke(
			testutil.Bob,
			testutil.Alice,
			blobcaps.Retrieve.New(
				testutil.Alice.DID().String(),
				blobcaps.RetrieveCaveats{
					Blob: blobcaps.Blob{Digest: blob.cid.Hash()},
				},
			),
			delegation.WithProof(prf),
		)
		require.NoError(t, err)

		xres, hres, err := retrievalclient.Execute(t.Context(), inv, conn)
		require.NoError(t, err)

		expectStatus := http.StatusOK
		expectHeaders := http.Header{
			http.CanonicalHeaderKey("Content-Length"): []string{fmt.Sprintf("%d", len(blob.bytes))},
		}
		expectBody := blob.bytes

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
