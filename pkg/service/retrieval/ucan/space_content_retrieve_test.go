package ucan

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	logging "github.com/ipfs/go-log/v2"
	"github.com/multiformats/go-multihash"
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
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/server/retrieval"
	"github.com/storacha/go-ucanto/transport/headercar"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/stretchr/testify/require"

	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/allocationstore/allocation"
	"github.com/storacha/piri/pkg/store/blobstore"
)

type retrievalService struct {
	allocations allocationstore.AllocationStore
	blobs       blobstore.BlobGetter
}

func (rs *retrievalService) Allocations() allocationstore.AllocationStore {
	return rs.allocations
}

func (rs *retrievalService) Blobs() blobstore.BlobGetter {
	return rs.blobs
}

func TestSpaceContentRetrieve(t *testing.T) {
	logging.SetLogLevel("retrieval/ucan", "DEBUG")
	alice := testutil.Alice
	space := testutil.RandomSigner(t)
	proof, err := delegation.Delegate(
		space,
		alice,
		[]ucan.Capability[ucan.NoCaveats]{
			ucan.NewCapability(
				content.RetrieveAbility,
				space.DID().String(),
				ucan.NoCaveats{},
			),
		},
	)
	require.NoError(t, err)
	otherSpace := testutil.RandomDID(t)

	randBytes := testutil.RandomBytes(t, 32)
	blob := struct {
		bytes  []byte
		digest multihash.Multihash
	}{randBytes, testutil.MultihashFromBytes(t, randBytes)}

	testCases := []struct {
		name          string
		agent         ucan.Signer
		space         did.DID
		proof         delegation.Delegation
		allocations   []allocation.Allocation
		blobs         [][]byte
		caveats       content.RetrieveCaveats
		expectStatus  int
		expectHeaders http.Header
		expectBody    []byte
		assertError   func(ipld.Node)
	}{
		{
			name:        "not found when missing allocation",
			agent:       alice,
			space:       space.DID(),
			proof:       proof,
			allocations: []allocation.Allocation{},
			blobs:       [][]byte{blob.bytes},
			caveats: content.RetrieveCaveats{
				Blob:  content.BlobDigest{Digest: blob.digest},
				Range: content.Range{Start: 0, End: 0},
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
			name:  "not found when missing blob",
			agent: alice,
			space: space.DID(),
			proof: proof,
			allocations: []allocation.Allocation{
				{
					Space: space.DID(),
					Blob: allocation.Blob{
						Digest: blob.digest,
						Size:   uint64(len(blob.digest)),
					},
					Expires: uint64(time.Now().Unix() + 30),
					Cause:   testutil.RandomCID(t),
				},
			},
			blobs: [][]byte{},
			caveats: content.RetrieveCaveats{
				Blob:  content.BlobDigest{Digest: blob.digest},
				Range: content.Range{Start: 0, End: 0},
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
			name:  "not found when allocation for other space",
			agent: alice,
			space: space.DID(),
			proof: proof,
			allocations: []allocation.Allocation{
				{
					Space: otherSpace,
					Blob: allocation.Blob{
						Digest: blob.digest,
						Size:   uint64(len(blob.digest)),
					},
					Expires: uint64(time.Now().Unix() + 30),
					Cause:   testutil.RandomCID(t),
				},
			},
			blobs: [][]byte{blob.bytes},
			caveats: content.RetrieveCaveats{
				Blob:  content.BlobDigest{Digest: blob.digest},
				Range: content.Range{Start: 0, End: 0},
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
			name:  "range end greater than blob size",
			agent: alice,
			space: space.DID(),
			proof: proof,
			allocations: []allocation.Allocation{
				{
					Space: space.DID(),
					Blob: allocation.Blob{
						Digest: blob.digest,
						Size:   uint64(len(blob.digest)),
					},
					Expires: uint64(time.Now().Unix() + 30),
					Cause:   testutil.RandomCID(t),
				},
			},
			blobs: [][]byte{blob.bytes},
			caveats: content.RetrieveCaveats{
				Blob:  content.BlobDigest{Digest: blob.digest},
				Range: content.Range{Start: 0, End: uint64(len(blob.bytes)) + 1},
			},
			expectStatus: http.StatusRequestedRangeNotSatisfiable,
			expectBody:   []byte{},
			assertError: func(n ipld.Node) {
				x, err := ipld.Rebind[content.RangeNotSatisfiableError](n, content.RangeNotSatisfiableErrorType())
				require.NoError(t, err)
				require.Equal(t, content.RangeNotSatisfiableErrorName, x.Name())
			},
		},
		{
			name:  "range end less than range start",
			agent: alice,
			space: space.DID(),
			proof: proof,
			allocations: []allocation.Allocation{
				{
					Space: space.DID(),
					Blob: allocation.Blob{
						Digest: blob.digest,
						Size:   uint64(len(blob.digest)),
					},
					Expires: uint64(time.Now().Unix() + 30),
					Cause:   testutil.RandomCID(t),
				},
			},
			blobs: [][]byte{blob.bytes},
			caveats: content.RetrieveCaveats{
				Blob:  content.BlobDigest{Digest: blob.digest},
				Range: content.Range{Start: 1, End: 0},
			},
			expectStatus: http.StatusRequestedRangeNotSatisfiable,
			expectBody:   []byte{},
			assertError: func(n ipld.Node) {
				x, err := ipld.Rebind[content.RangeNotSatisfiableError](n, content.RangeNotSatisfiableErrorType())
				require.NoError(t, err)
				require.Equal(t, content.RangeNotSatisfiableErrorName, x.Name())
			},
		},
		{
			name:  "retrieve all",
			agent: alice,
			space: space.DID(),
			proof: proof,
			allocations: []allocation.Allocation{
				{
					Space: space.DID(),
					Blob: allocation.Blob{
						Digest: blob.digest,
						Size:   uint64(len(blob.digest)),
					},
					Expires: uint64(time.Now().Unix() + 30),
					Cause:   testutil.RandomCID(t),
				},
			},
			blobs: [][]byte{blob.bytes},
			caveats: content.RetrieveCaveats{
				Blob:  content.BlobDigest{Digest: blob.digest},
				Range: content.Range{Start: 0, End: uint64(len(blob.bytes) - 1)},
			},
			expectStatus: http.StatusOK,
			expectHeaders: http.Header{
				http.CanonicalHeaderKey("Content-Length"): []string{fmt.Sprintf("%d", len(blob.bytes))},
			},
			expectBody: blob.bytes,
		},
		{
			name:  "retrieve partial",
			agent: alice,
			space: space.DID(),
			proof: proof,
			allocations: []allocation.Allocation{
				{
					Space: space.DID(),
					Blob: allocation.Blob{
						Digest: blob.digest,
						Size:   uint64(len(blob.digest)),
					},
					Expires: uint64(time.Now().Unix() + 30),
					Cause:   testutil.RandomCID(t),
				},
			},
			blobs: [][]byte{blob.bytes},
			caveats: content.RetrieveCaveats{
				Blob:  content.BlobDigest{Digest: blob.digest},
				Range: content.Range{Start: 0, End: 1},
			},
			expectStatus: http.StatusPartialContent,
			expectHeaders: http.Header{
				http.CanonicalHeaderKey("Content-Length"): []string{fmt.Sprintf("%d", 2)},
				http.CanonicalHeaderKey("Content-Range"):  []string{fmt.Sprintf("bytes %d-%d/%d", 0, 1, len(blob.bytes))},
			},
			expectBody: blob.bytes[0:2],
		},
		{
			name:  "bad proof",
			agent: alice,
			space: otherSpace,
			proof: proof,
			allocations: []allocation.Allocation{
				{
					Space: otherSpace,
					Blob: allocation.Blob{
						Digest: blob.digest,
						Size:   uint64(len(blob.digest)),
					},
					Expires: uint64(time.Now().Unix() + 30),
					Cause:   testutil.RandomCID(t),
				},
			},
			blobs: [][]byte{blob.bytes},
			caveats: content.RetrieveCaveats{
				Blob:  content.BlobDigest{Digest: blob.digest},
				Range: content.Range{Start: 0, End: 0},
			},
			expectStatus: http.StatusOK,
			expectBody:   []byte{},
			assertError: func(n ipld.Node) {
				x, err := ipld.Rebind[fdm.FailureModel](n, fdm.FailureType())
				require.NoError(t, err)
				require.Equal(t, "Unauthorized", *x.Name)
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			allocations := allocationstore.NewDatastoreStore(sync.MutexWrap(datastore.NewMapDatastore()))
			for _, a := range test.allocations {
				err := allocations.Put(t.Context(), a)
				require.NoError(t, err)
			}

			blobs := blobstore.NewDatastoreStore(sync.MutexWrap(datastore.NewMapDatastore()))
			for _, b := range test.blobs {
				digest, err := multihash.Sum(b, multihash.SHA2_256, -1)
				require.NoError(t, err)
				err = blobs.Put(t.Context(), digest, uint64(len(b)), bytes.NewReader(b))
				require.NoError(t, err)
			}

			service := retrievalService{allocations, blobs}
			server, err := retrieval.NewServer(testutil.Service, WithSpaceContentRetrieveMethod(&service))
			require.NoError(t, err)

			inv, err := invocation.Invoke(
				test.agent,
				testutil.Service,
				content.Retrieve.New(test.space.String(), test.caveats),
				delegation.WithProof(delegation.FromDelegation(test.proof)),
			)
			require.NoError(t, err)

			codecOpt := client.WithOutboundCodec(headercar.NewOutboundCodec())
			conn, err := client.NewConnection(testutil.Service, server, codecOpt)
			require.NoError(t, err)

			xres, hres, err := rclient.Execute(t.Context(), inv, conn)
			require.NoError(t, err)

			require.Equal(t, test.expectStatus, hres.Status())
			for k, v := range test.expectHeaders {
				require.Equal(t, v, hres.Headers().Values(k))
			}
			require.Equal(t, test.expectBody, testutil.Must(io.ReadAll(hres.Body()))(t))

			rcptLink, ok := xres.Get(inv.Link())
			require.True(t, ok)

			rcpt, err := receipt.NewAnyReceiptReader().Read(rcptLink, xres.Blocks())
			require.NoError(t, err)

			_, x := result.Unwrap(rcpt.Out())
			if test.assertError != nil {
				test.assertError(x)
			} else {
				require.Nil(t, x)
			}
		})
	}
}
