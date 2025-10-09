package main

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"github.com/ipni/go-libipni/ingest/schema"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/ipnipublisher/publisher"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-libstoracha/metadata"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/stretchr/testify/require"
)

func TestValidateAdvertSig(t *testing.T) {
	sk0, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)

	ad := schema.Advertisement{
		Entries: testutil.RandomCID(t),
	}
	err = ad.Sign(sk0)
	require.NoError(t, err)

	err = validateAdvertSig(sk0, ad)
	require.NoError(t, err)

	sk1, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)

	err = validateAdvertSig(sk1, ad)
	require.Error(t, err)
	require.Contains(t, err.Error(), "advert was not created by this node")
}

func TestDecodeAdvert(t *testing.T) {
	f, err := os.Open("./testdata/advert.json")
	require.NoError(t, err)
	defer f.Close()

	ad, err := decodeAdvert(f)
	require.NoError(t, err)
	require.NotEmpty(t, ad)
}

func TestPublishAdvert(t *testing.T) {
	f, err := os.Open("./testdata/advert.json")
	require.NoError(t, err)
	defer f.Close()

	ad, err := decodeAdvert(f)
	require.NoError(t, err)

	sk, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)

	publisherStore := store.FromDatastore(
		sync.MutexWrap(datastore.NewMapDatastore()),
		store.WithMetadataContext(metadata.MetadataContext),
	)

	// we need at least one advert already to be able to call [publishAdvert]
	pub, err := publisher.New(sk, publisherStore)
	require.NoError(t, err)

	ctxID := string(testutil.RandomBytes(t, 32))
	md := metadata.MetadataContext.New()
	providerInfo := peer.AddrInfo{
		ID:    testutil.RandomPeer(t),
		Addrs: []multiaddr.Multiaddr{testutil.RandomMultiaddr(t)},
	}
	digests := []multihash.Multihash{testutil.RandomMultihash(t)}

	_, err = pub.Publish(t.Context(), providerInfo, ctxID, slices.Values(digests), md)
	require.NoError(t, err)

	adlink, err := publishAdvert(t.Context(), sk, publisherStore, ad)
	require.NoError(t, err)

	out, err := json.Marshal(adlink)
	require.NoError(t, err)
	t.Log(string(out))
}
