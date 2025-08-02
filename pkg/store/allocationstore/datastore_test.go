package allocationstore

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/piri/pkg/store/allocationstore/allocation"
	"github.com/stretchr/testify/require"
)

func TestDsAllocationStore(t *testing.T) {
	t.Run("roundtrip", func(t *testing.T) {
		store, err := NewDsAllocationStore(datastore.NewMapDatastore())
		require.NoError(t, err)

		alloc := allocation.Allocation{
			Space: testutil.RandomDID(t),
			Blob: allocation.Blob{
				Digest: testutil.RandomMultihash(t),
				Size:   uint64(1 + rand.IntN(1000)),
			},
			Expires: uint64(time.Now().Unix()),
			Cause:   testutil.RandomCID(t),
		}

		err = store.Put(t.Context(), alloc)
		require.NoError(t, err)

		allocs, err := store.List(t.Context(), alloc.Blob.Digest)
		require.NoError(t, err)
		require.Len(t, allocs, 1)
		require.Equal(t, alloc, allocs[0])
	})

	t.Run("multiple", func(t *testing.T) {
		store, err := NewDsAllocationStore(datastore.NewMapDatastore())
		require.NoError(t, err)

		alloc0 := allocation.Allocation{
			Space: testutil.RandomDID(t),
			Blob: allocation.Blob{
				Digest: testutil.RandomMultihash(t),
				Size:   uint64(1 + rand.IntN(1000)),
			},
			Expires: uint64(time.Now().Unix()),
			Cause:   testutil.RandomCID(t),
		}

		alloc1 := allocation.Allocation{
			Space:   testutil.RandomDID(t),
			Blob:    alloc0.Blob,
			Expires: uint64(time.Now().Unix()),
			Cause:   testutil.RandomCID(t),
		}

		err = store.Put(t.Context(), alloc0)
		require.NoError(t, err)
		err = store.Put(t.Context(), alloc1)
		require.NoError(t, err)

		allocs, err := store.List(t.Context(), alloc0.Blob.Digest)
		require.NoError(t, err)
		require.Len(t, allocs, 2)

		if alloc0.Space.String() == allocs[0].Space.String() {
			require.Equal(t, []allocation.Allocation{alloc0, alloc1}, allocs)
		} else {
			require.Equal(t, []allocation.Allocation{alloc1, alloc0}, allocs)
		}
	})
}
