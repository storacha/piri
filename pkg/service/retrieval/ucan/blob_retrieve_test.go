package ucan

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	logging "github.com/ipfs/go-log/v2"
	"github.com/multiformats/go-multihash"
	blobcaps "github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-libstoracha/capabilities/space/content"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/go-ucanto/client"
	rclient "github.com/storacha/go-ucanto/client/retrieval"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/ipld"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/core/result"
	fdm "github.com/storacha/go-ucanto/core/result/failure/datamodel"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/server/retrieval"
	"github.com/storacha/go-ucanto/transport/headercar"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/stretchr/testify/require"
)

type blobRetrievalService struct {
	id    principal.Signer
	blobs blobstore.BlobGetter
}

func (brs *blobRetrievalService) ID() principal.Signer {
	return brs.id
}

func (brs *blobRetrievalService) Blobs() blobstore.BlobGetter {
	return brs.blobs
}

func TestBlobRetrieve(t *testing.T) {
	logging.SetLogLevel("retrieval/ucan", "DEBUG")
	alice := testutil.Alice
	proof, err := delegation.Delegate(
		testutil.Service,
		alice,
		[]ucan.Capability[ucan.NoCaveats]{
			ucan.NewCapability(
				blobcaps.RetrieveAbility,
				testutil.Service.DID().String(),
				ucan.NoCaveats{},
			),
		},
	)
	require.NoError(t, err)

	randBytes := testutil.RandomBytes(t, 32)
	blob := struct {
		bytes  []byte
		digest multihash.Multihash
	}{randBytes, testutil.MultihashFromBytes(t, randBytes)}

	testCases := []struct {
		name          string
		agent         ucan.Signer
		resource      ucan.Resource
		proof         delegation.Delegation
		blobs         [][]byte
		caveats       blobcaps.RetrieveCaveats
		expectStatus  int
		expectHeaders http.Header
		expectBody    []byte
		assertError   func(ipld.Node)
	}{
		{
			name:     "not found when missing blob",
			agent:    alice,
			resource: testutil.Service.DID().String(),
			proof:    proof,
			blobs:    [][]byte{},
			caveats: blobcaps.RetrieveCaveats{
				Blob: blobcaps.Blob{Digest: blob.digest},
			},
			expectStatus: http.StatusNotFound,
			expectBody:   []byte{},
			assertError: func(n ipld.Node) {
				x, err := ipld.Rebind[content.NotFoundError](n, content.NotFoundErrorType())
				require.NoError(t, err)
				require.Equal(t, content.NotFoundErrorName, x.Name())
			},
		},
		{
			name:     "bad proof",
			agent:    alice,
			resource: testutil.Service.DID().String(),
			proof: testutil.Must(
				delegation.Delegate(
					testutil.Bob,
					alice,
					[]ucan.Capability[ucan.NoCaveats]{
						ucan.NewCapability(
							blobcaps.RetrieveAbility,
							testutil.Service.DID().String(),
							ucan.NoCaveats{},
						),
					},
				),
			)(t),
			blobs: [][]byte{blob.bytes},
			caveats: blobcaps.RetrieveCaveats{
				Blob: blobcaps.Blob{Digest: blob.digest},
			},
			expectStatus: http.StatusOK,
			expectBody:   []byte{},
			assertError: func(n ipld.Node) {
				x, err := ipld.Rebind[fdm.FailureModel](n, fdm.FailureType())
				require.NoError(t, err)
				require.Equal(t, "Unauthorized", *x.Name)
			},
		},
		{
			name:     "wrong resource",
			agent:    alice,
			resource: testutil.Mallory.DID().String(),
			proof: testutil.Must(
				delegation.Delegate(
					testutil.Mallory,
					alice,
					[]ucan.Capability[ucan.NoCaveats]{
						ucan.NewCapability(
							blobcaps.RetrieveAbility,
							testutil.Mallory.DID().String(),
							ucan.NoCaveats{},
						),
					},
				),
			)(t),
			blobs: [][]byte{blob.bytes},
			caveats: blobcaps.RetrieveCaveats{
				Blob: blobcaps.Blob{Digest: blob.digest},
			},
			expectStatus: http.StatusOK,
			expectBody:   []byte{},
			assertError: func(n ipld.Node) {
				x, err := ipld.Rebind[fdm.FailureModel](n, fdm.FailureType())
				require.NoError(t, err)
				require.Equal(t, InvalidResourceErrorName, *x.Name)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			blobs := blobstore.NewMapBlobstore()
			for _, b := range tc.blobs {
				digest, err := multihash.Sum(b, multihash.SHA2_256, -1)
				require.NoError(t, err)
				err = blobs.Put(t.Context(), digest, uint64(len(b)), bytes.NewReader(b))
				require.NoError(t, err)
			}

			service := blobRetrievalService{testutil.Service, blobs}
			server, err := retrieval.NewServer(testutil.Service, BlobRetrieve(&service))
			require.NoError(t, err)

			inv, err := invocation.Invoke(
				tc.agent,
				testutil.Service,
				blobcaps.Retrieve.New(tc.resource, tc.caveats),
				delegation.WithProof(delegation.FromDelegation(tc.proof)),
			)
			require.NoError(t, err)

			codecOpt := client.WithOutboundCodec(headercar.NewOutboundCodec())
			conn, err := client.NewConnection(testutil.Service, server, codecOpt)
			require.NoError(t, err)

			xres, hres, err := rclient.Execute(t.Context(), inv, conn)
			require.NoError(t, err)

			require.Equal(t, tc.expectStatus, hres.Status())
			for k, v := range tc.expectHeaders {
				require.Equal(t, v, hres.Headers().Values(k))
			}
			require.Equal(t, tc.expectBody, testutil.Must(io.ReadAll(hres.Body()))(t))

			rcptLink, ok := xres.Get(inv.Link())
			require.True(t, ok)

			rcpt, err := receipt.NewAnyReceiptReader().Read(rcptLink, xres.Blocks())
			require.NoError(t, err)

			_, x := result.Unwrap(rcpt.Out())
			if tc.assertError != nil {
				tc.assertError(x)
			} else {
				require.Nil(t, x)
			}
		})
	}
}
