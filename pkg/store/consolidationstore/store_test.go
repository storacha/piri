package consolidationstore

import (
	"io"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/result/ok"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/stretchr/testify/require"

	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/consolidationstore/consolidation"
)

func TestDatastoreConsolidationStore(t *testing.T) {
	t.Run("roundtrip", func(t *testing.T) {
		ds := datastore.NewMapDatastore()
		s := NewDatastoreStore(ds)

		batchCID := randomCID(t)
		c := createTestConsolidation(t)

		err := s.Put(t.Context(), batchCID, c)
		require.NoError(t, err)

		got, err := s.Get(t.Context(), batchCID)
		require.NoError(t, err)
		requireEqualConsolidation(t, c, got)
	})

	t.Run("not found", func(t *testing.T) {
		ds := datastore.NewMapDatastore()
		s := NewDatastoreStore(ds)

		batchCID := randomCID(t)

		_, err := s.Get(t.Context(), batchCID)
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("delete", func(t *testing.T) {
		ds := datastore.NewMapDatastore()
		s := NewDatastoreStore(ds)

		batchCID := randomCID(t)
		c := createTestConsolidation(t)

		// Put
		err := s.Put(t.Context(), batchCID, c)
		require.NoError(t, err)

		// Verify exists
		got, err := s.Get(t.Context(), batchCID)
		require.NoError(t, err)
		requireEqualConsolidation(t, c, got)

		// Delete
		err = s.Delete(t.Context(), batchCID)
		require.NoError(t, err)

		// Verify not found
		_, err = s.Get(t.Context(), batchCID)
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("delete non-existent", func(t *testing.T) {
		ds := datastore.NewMapDatastore()
		s := NewDatastoreStore(ds)

		batchCID := randomCID(t)

		// Delete should not error on non-existent key
		err := s.Delete(t.Context(), batchCID)
		require.NoError(t, err)
	})

	t.Run("overwrite", func(t *testing.T) {
		ds := datastore.NewMapDatastore()
		s := NewDatastoreStore(ds)

		batchCID := randomCID(t)
		c1 := createTestConsolidation(t)
		c2 := createTestConsolidation(t)

		// Put first
		err := s.Put(t.Context(), batchCID, c1)
		require.NoError(t, err)

		// Overwrite with second
		err = s.Put(t.Context(), batchCID, c2)
		require.NoError(t, err)

		// Get should return second
		got, err := s.Get(t.Context(), batchCID)
		require.NoError(t, err)
		requireEqualConsolidation(t, c2, got)
	})

	t.Run("legacy migration", func(t *testing.T) {
		ds := datastore.NewMapDatastore()

		// Create test consolidation
		c := createTestConsolidation(t)
		batchCID := randomCID(t)

		// Write to legacy format directly
		trackDS := namespace.Wrap(ds, datastore.NewKey(legacyTrackPrefix))
		consolidateDS := namespace.Wrap(ds, datastore.NewKey(legacyConsolidatePrefix))

		key := datastore.NewKey(batchCID.String())

		// Archive the track invocation to CAR bytes
		trackBytes, err := io.ReadAll(c.TrackInvocation.Archive())
		require.NoError(t, err)
		err = trackDS.Put(t.Context(), key, trackBytes)
		require.NoError(t, err)

		// Write consolidate CID bytes
		err = consolidateDS.Put(t.Context(), key, c.ConsolidateInvocationCID.Bytes())
		require.NoError(t, err)

		// Create store and get - should read from legacy and migrate
		s := NewDatastoreStore(ds)
		got, err := s.Get(t.Context(), batchCID)
		require.NoError(t, err)
		requireEqualConsolidation(t, c, got)

		// Verify legacy data was cleaned up
		_, err = trackDS.Get(t.Context(), key)
		require.ErrorIs(t, err, datastore.ErrNotFound)

		_, err = consolidateDS.Get(t.Context(), key)
		require.ErrorIs(t, err, datastore.ErrNotFound)

		// Verify data is now in new format (can be retrieved again)
		got2, err := s.Get(t.Context(), batchCID)
		require.NoError(t, err)
		requireEqualConsolidation(t, c, got2)
	})
}

func createTestConsolidation(t *testing.T) consolidation.Consolidation {
	t.Helper()

	signer := testutil.RandomSigner(t)
	audience := testutil.RandomDID(t)

	inv, err := delegation.Delegate(
		signer,
		audience,
		[]ucan.Capability[ok.Unit]{
			ucan.NewCapability("space/egress/track", audience.String(), ok.Unit{}),
		},
	)
	require.NoError(t, err)

	return consolidation.Consolidation{
		TrackInvocation:          inv,
		ConsolidateInvocationCID: randomCID(t),
	}
}

func randomCID(t *testing.T) cid.Cid {
	t.Helper()
	link := testutil.RandomCID(t)
	return link.(cidlink.Link).Cid
}

func requireEqualConsolidation(t *testing.T, expected, actual consolidation.Consolidation) {
	t.Helper()

	// Compare invocation links (the canonical identifier)
	require.Equal(t, expected.TrackInvocation.Link(), actual.TrackInvocation.Link())
	// Compare consolidate CIDs
	require.Equal(t, expected.ConsolidateInvocationCID, actual.ConsolidateInvocationCID)
}
