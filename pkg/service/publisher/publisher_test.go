package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/result/failure"
	"github.com/storacha/go-ucanto/core/result/ok"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/principal/signer"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/storacha/piri/pkg/internal/digestutil"
	"github.com/storacha/piri/pkg/principalresolver"
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
		handler := func(ctx context.Context, cap ucan.Capability[claim.CacheCaveats], inv invocation.Invocation, context server.InvocationContext) (result.Result[ok.Unit, failure.IPLDBuilderFailure], fx.Effects, error) {
			handlerCalled = true
			claim := cap.Nb().Claim
			for b, err := range inv.Blocks() {
				if err != nil {
					return nil, nil, err
				}
				if b.Link() == claim {
					return result.Ok[ok.Unit, failure.IPLDBuilderFailure](ok.Unit{}), nil, nil
				}
			}
			return nil, nil, fmt.Errorf("claim not found in invocation blocks: %s", claim.String())
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

func mockIndexingService(t *testing.T, id principal.Signer, handler server.HandlerFunc[claim.CacheCaveats, ok.Unit, failure.IPLDBuilderFailure]) server.ServerView {
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

func TestValidateClaimCacheDelegation(t *testing.T) {
	var serviceID principal.Signer
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != principalresolver.WellKnownDIDPath {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		doc := principalresolver.Document{
			Context: principalresolver.FlexibleContext{"https://w3id.org/did/v1", "https://w3id.org/security/v1"},
			ID:      serviceID.DID().String(),
			VerificationMethod: []principalresolver.VerificationMethod{
				{
					ID:                 serviceID.DID().String() + "#key1",
					Type:               "Ed25519VerificationKey2018",
					Controller:         serviceID.DID().String(),
					PublicKeyMultibase: strings.TrimPrefix(testutil.Service.DID().String(), "did:key:"),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(doc)
	}))
	defer server.Close()

	didWeb, err := did.Parse("did:web:" + testutil.Must(url.Parse(server.URL))(t).Host)
	require.NoError(t, err)

	serviceID, err = signer.Wrap(testutil.Service, didWeb)
	require.NoError(t, err)

	dlg, err := delegation.Delegate(
		serviceID,
		testutil.Alice,
		[]ucan.Capability[ucan.NoCaveats]{
			ucan.NewCapability(claim.CacheAbility, serviceID.DID().String(), ucan.NoCaveats{}),
		},
	)
	require.NoError(t, err)

	proofs := []delegation.Proof{delegation.FromDelegation(dlg)}

	err = validateClaimCacheDelegation(t.Context(), testutil.Alice, serviceID, proofs, principalresolver.InsecureResolution())
	require.NoError(t, err)
}
