package service

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/go-data-segment/merkletree"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/multiformats/go-multihash"
	blobcaps "github.com/storacha/go-libstoracha/capabilities/blob"
	pdpcaps "github.com/storacha/go-libstoracha/capabilities/pdp"
	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/digestutil"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/go-ucanto/core/ipld"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/core/receipt/ran"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/stretchr/testify/require"

	"github.com/storacha/piri/pkg/store/acceptancestore"
	"github.com/storacha/piri/pkg/store/acceptancestore/acceptance"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

func TestGetAddPieceProofs(t *testing.T) {
	space := testutil.RandomSigner(t)
	size := 256
	blob := testutil.RandomMultihash(t)
	piece := testutil.RandomPiece(t, int64(size))
	aggregate := testutil.RandomPiece(t, 256*1024*1024)

	blobAccInv, err := blobcaps.Accept.Invoke(
		testutil.WebService,
		testutil.Alice,
		testutil.Alice.DID().String(),
		blobcaps.AcceptCaveats{
			Space: space.DID(),
			Blob: types.Blob{
				Digest: blob,
				Size:   uint64(size),
			},
			Put: blobcaps.Promise{
				UcanAwait: blobcaps.Await{
					Selector: ".out.ok",
					Link:     testutil.RandomCID(t),
				},
			},
		},
	)
	require.NoError(t, err)

	pdpAccInv, err := pdpcaps.Accept.Invoke(
		testutil.Alice,
		testutil.Alice,
		testutil.Alice.DID().String(),
		pdpcaps.AcceptCaveats{
			Blob: blob,
		},
	)
	require.NoError(t, err)

	pdpAccInvLink := pdpAccInv.Link()

	blobAccRcpt, err := receipt.Issue(
		testutil.Alice,
		result.Ok[blobcaps.AcceptOk, ipld.Builder](blobcaps.AcceptOk{
			Site: testutil.RandomCID(t),
			PDP:  &pdpAccInvLink,
		}),
		ran.FromInvocation(blobAccInv),
	)
	require.NoError(t, err)

	pdpAccRcpt, err := receipt.Issue(
		testutil.Alice,
		result.Ok[pdpcaps.AcceptOk, ipld.Builder](pdpcaps.AcceptOk{
			Piece:          piece,
			Aggregate:      aggregate,
			InclusionProof: merkletree.ProofData{},
		}),
		ran.FromInvocation(pdpAccInv),
	)
	require.NoError(t, err)

	resolver := mockResolver{
		map[string]multihash.Multihash{
			digestutil.Format(piece.Link().(cidlink.Link).Cid.Hash()): blob,
		},
	}

	accStore := acceptancestore.NewDatastoreStore(datastore.NewMapDatastore())

	err = accStore.Put(t.Context(), acceptance.Acceptance{
		Space: space.DID(),
		Blob: acceptance.Blob{
			Digest: blob,
			Size:   uint64(size),
		},
		PDPAccept: &acceptance.Promise{
			UcanAwait: acceptance.Await{
				Selector: ".out.ok",
				Link:     pdpAccInv.Link(),
			},
		},
		ExecutedAt: uint64(time.Now().Unix()),
		Cause:      blobAccInv.Link(),
	})
	require.NoError(t, err)

	rcptStore := receiptstore.NewDatastoreStore(sync.MutexWrap(datastore.NewMapDatastore()))
	err = rcptStore.Put(t.Context(), blobAccRcpt)
	require.NoError(t, err)

	err = rcptStore.Put(t.Context(), pdpAccRcpt)
	require.NoError(t, err)

	task, msg, err := getAddPieceProofs(t.Context(), &resolver, accStore, rcptStore, piece.Link().(cidlink.Link).Cid)
	require.NoError(t, err)

	require.Equal(t, blobAccInv.Link(), task)

	_, ok, err := msg.Invocation(blobAccInv.Link())
	require.NoError(t, err)
	require.True(t, ok)

	_, ok, err = msg.Invocation(pdpAccInvLink)
	require.NoError(t, err)
	require.True(t, ok)

	_, ok, err = msg.Receipt(blobAccRcpt.Root().Link())
	require.NoError(t, err)
	require.True(t, ok)

	_, ok, err = msg.Receipt(pdpAccRcpt.Root().Link())
	require.NoError(t, err)
	require.True(t, ok)
}

type mockResolver struct {
	data map[string]multihash.Multihash
}

func (r *mockResolver) ResolveToBlob(ctx context.Context, piece multihash.Multihash) (multihash.Multihash, bool, error) {
	key := digestutil.Format(piece)
	blobDigest, ok := r.data[key]
	return blobDigest, ok, nil
}
