package allocationstore

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/stretchr/testify/require"

	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/allocationstore/allocation"
)

func TestDatastoreAllocationStore(t *testing.T) {
	t.Run("roundtrip", func(t *testing.T) {
		s := NewDatastoreStore(datastore.NewMapDatastore())

		alloc := allocation.Allocation{
			Space: testutil.RandomDID(t),
			Blob: allocation.Blob{
				Digest: testutil.RandomMultihash(t),
				Size:   uint64(1 + rand.IntN(1000)),
			},
			Expires: uint64(time.Now().Unix()),
			Cause:   testutil.RandomCID(t),
		}

		err := s.Put(t.Context(), alloc)
		require.NoError(t, err)

		got, err := s.Get(t.Context(), alloc.Blob.Digest, alloc.Space)
		require.NoError(t, err)
		require.Equal(t, alloc, got)
	})

	t.Run("get any", func(t *testing.T) {
		s := NewDatastoreStore(datastore.NewMapDatastore())

		alloc := allocation.Allocation{
			Space: testutil.RandomDID(t),
			Blob: allocation.Blob{
				Digest: testutil.RandomMultihash(t),
				Size:   uint64(1 + rand.IntN(1000)),
			},
			Expires: uint64(time.Now().Unix()),
			Cause:   testutil.RandomCID(t),
		}

		err := s.Put(t.Context(), alloc)
		require.NoError(t, err)

		got, err := s.GetAny(t.Context(), alloc.Blob.Digest)
		require.NoError(t, err)
		require.Equal(t, alloc, got)
	})

	t.Run("exists", func(t *testing.T) {
		s := NewDatastoreStore(datastore.NewMapDatastore())

		alloc := allocation.Allocation{
			Space: testutil.RandomDID(t),
			Blob: allocation.Blob{
				Digest: testutil.RandomMultihash(t),
				Size:   uint64(1 + rand.IntN(1000)),
			},
			Expires: uint64(time.Now().Unix()),
			Cause:   testutil.RandomCID(t),
		}

		exists, err := s.Exists(t.Context(), alloc.Blob.Digest)
		require.NoError(t, err)
		require.False(t, exists)

		err = s.Put(t.Context(), alloc)
		require.NoError(t, err)

		exists, err = s.Exists(t.Context(), alloc.Blob.Digest)
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("multiple spaces same blob", func(t *testing.T) {
		s := NewDatastoreStore(datastore.NewMapDatastore())

		blob := allocation.Blob{
			Digest: testutil.RandomMultihash(t),
			Size:   uint64(1 + rand.IntN(1000)),
		}

		alloc0 := allocation.Allocation{
			Space:   testutil.RandomDID(t),
			Blob:    blob,
			Expires: uint64(time.Now().Unix()),
			Cause:   testutil.RandomCID(t),
		}

		alloc1 := allocation.Allocation{
			Space:   testutil.RandomDID(t),
			Blob:    blob,
			Expires: uint64(time.Now().Unix()),
			Cause:   testutil.RandomCID(t),
		}

		err := s.Put(t.Context(), alloc0)
		require.NoError(t, err)
		err = s.Put(t.Context(), alloc1)
		require.NoError(t, err)

		// Get specific allocations
		got0, err := s.Get(t.Context(), blob.Digest, alloc0.Space)
		require.NoError(t, err)
		require.Equal(t, alloc0, got0)

		got1, err := s.Get(t.Context(), blob.Digest, alloc1.Space)
		require.NoError(t, err)
		require.Equal(t, alloc1, got1)

		// GetAny returns one of them
		gotAny, err := s.GetAny(t.Context(), blob.Digest)
		require.NoError(t, err)
		require.True(t, gotAny.Space == alloc0.Space || gotAny.Space == alloc1.Space)

		// Exists returns true
		exists, err := s.Exists(t.Context(), blob.Digest)
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("not found", func(t *testing.T) {
		s := NewDatastoreStore(datastore.NewMapDatastore())

		digest := testutil.RandomMultihash(t)
		space := testutil.RandomDID(t)

		_, err := s.Get(t.Context(), digest, space)
		require.ErrorIs(t, err, store.ErrNotFound)

		_, err = s.GetAny(t.Context(), digest)
		require.ErrorIs(t, err, store.ErrNotFound)
	})
}
