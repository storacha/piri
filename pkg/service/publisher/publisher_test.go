package publisher

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"

	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/capabilities/assert"
	"github.com/storacha/go-libstoracha/capabilities/claim"
	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-libstoracha/metadata"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/result/ok"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/storacha/piri/pkg/internal/digestutil"
	"github.com/storacha/piri/pkg/internal/testutil"
	"github.com/storacha/piri/pkg/service/publisher/advertisement"
	"github.com/stretchr/testify/require"
)

func TestPublisherService(t *testing.T) {
	addr, err := multiaddr.NewMultiaddr("/dns4/localhost/tcp/3000/http")
	require.NoError(t, err)

	ctx := t.Context()

	t.Run("publishes location commitments", func(t *testing.T) {
		dstore := dssync.MutexWrap(datastore.NewMapDatastore())
		publisherStore := store.FromDatastore(dstore, store.WithMetadataContext(metadata.MetadataContext))

		svc, err := New(testutil.Alice, publisherStore, addr, WithLogLevel("info"))
		require.NoError(t, err)

		space := testutil.RandomDID(t)
		shard := testutil.RandomMultihash(t)
		location := testutil.Must(url.Parse(fmt.Sprintf("http://localhost:3000/blob/%s", digestutil.Format(shard))))(t)

		claim, err := assert.Location.Delegate(
			testutil.Alice,
			space,
			testutil.Alice.DID().String(),
			assert.LocationCaveats{
				Space:    space,
				Content:  types.FromHash(shard),
				Location: []url.URL{*location},
			},
			delegation.WithNoExpiration(),
		)
		require.NoError(t, err)

		err = svc.Publish(ctx, claim)
		require.NoError(t, err)

		hd, err := publisherStore.Head(ctx)
		require.NoError(t, err)

		ad, err := publisherStore.Advert(ctx, hd.Head)
		require.NoError(t, err)

		require.Equal(
			t,
			testutil.Must(advertisement.EncodeContextID(space, shard))(t),
			ad.ContextID,
		)

		meta := metadata.MetadataContext.New()
		err = meta.UnmarshalBinary(ad.Metadata)
		require.NoError(t, err)

		protocol := meta.Get(metadata.LocationCommitmentID)
		require.NotNil(t, protocol)

		lcmeta, ok := protocol.(*metadata.LocationCommitmentMetadata)
		require.True(t, ok)

		require.Equal(t, claim.Link().String(), lcmeta.Claim.String())

		var ents []multihash.Multihash
		for digest, err := range publisherStore.Entries(ctx, ad.Entries) {
			require.NoError(t, err)
			ents = append(ents, digest)
		}
		require.Len(t, ents, 1)
		require.Equal(t, shard, ents[0])
	})

	t.Run("allow skip publish existing advert", func(t *testing.T) {
		dstore := dssync.MutexWrap(datastore.NewMapDatastore())
		publisherStore := store.FromDatastore(dstore, store.WithMetadataContext(metadata.MetadataContext))

		svc, err := New(testutil.Alice, publisherStore, addr, WithLogLevel("info"))
		require.NoError(t, err)

		space := testutil.RandomDID(t)
		shard := testutil.RandomMultihash(t)
		location := testutil.Must(url.Parse(fmt.Sprintf("http://localhost:3000/blob/%s", digestutil.Format(shard))))(t)

		claim, err := assert.Location.Delegate(
			testutil.Alice,
			space,
			testutil.Alice.DID().String(),
			assert.LocationCaveats{
				Space:    space,
				Content:  types.FromHash(shard),
				Location: []url.URL{*location},
			},
			delegation.WithNoExpiration(),
		)
		require.NoError(t, err)

		err = svc.Publish(ctx, claim)
		require.NoError(t, err)

		err = svc.Publish(ctx, claim)
		require.NoError(t, err)
	})

	t.Run("caches claims", func(t *testing.T) {
		dstore := dssync.MutexWrap(datastore.NewMapDatastore())
		publisherStore := store.FromDatastore(dstore, store.WithMetadataContext(metadata.MetadataContext))

		handlerCalled := false
		handler := func(ctx context.Context, cap ucan.Capability[claim.CacheCaveats], inv invocation.Invocation, context server.InvocationContext) (ok.Unit, fx.Effects, error) {
			handlerCalled = true
			claim := cap.Nb().Claim
			for b, err := range inv.Blocks() {
				if err != nil {
					return ok.Unit{}, nil, err
				}
				if b.Link() == claim {
					return ok.Unit{}, nil, nil
				}
			}
			return ok.Unit{}, nil, fmt.Errorf("claim not found in invocation blocks: %s", claim.String())
		}

		idxSvc := mockIndexingService(t, testutil.Bob, handler)
		idxConn, err := client.NewConnection(testutil.Bob, idxSvc)
		require.NoError(t, err)

		// authorize alice to cache claim on bob
		prf, err := delegation.Delegate(
			testutil.Bob,
			testutil.Alice,
			[]ucan.Capability[ucan.NoCaveats]{
				ucan.NewCapability(
					claim.CacheAbility,
					testutil.Bob.DID().String(),
					ucan.NoCaveats{},
				),
			},
		)
		require.NoError(t, err)

		svc, err := New(
			testutil.Alice,
			publisherStore,
			addr,
			WithIndexingService(idxConn),
			WithIndexingServiceProof(delegation.FromDelegation(prf)),
			WithLogLevel("info"),
		)
		require.NoError(t, err)

		space := testutil.RandomDID(t)
		shard := testutil.RandomMultihash(t)
		location := testutil.Must(url.Parse(fmt.Sprintf("http://localhost:3000/blob/%s", digestutil.Format(shard))))(t)

		claim, err := assert.Location.Delegate(
			testutil.Alice,
			space,
			testutil.Alice.DID().String(),
			assert.LocationCaveats{
				Space:    space,
				Content:  types.FromHash(shard),
				Location: []url.URL{*location},
			},
			delegation.WithNoExpiration(),
		)
		require.NoError(t, err)

		err = svc.Publish(ctx, claim)
		require.NoError(t, err)
		require.True(t, handlerCalled)
	})
}

func mockIndexingService(t *testing.T, id principal.Signer, handler server.HandlerFunc[claim.CacheCaveats, ok.Unit]) server.ServerView {
	t.Helper()
	return testutil.Must(
		server.NewServer(
			id,
			server.WithServiceMethod(
				claim.CacheAbility,
				server.Provide(
					claim.Cache,
					handler,
				),
			),
		),
	)(t)
}
