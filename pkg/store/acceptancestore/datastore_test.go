package acceptancestore_test

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/piri/pkg/store/acceptancestore"
	"github.com/storacha/piri/pkg/store/acceptancestore/acceptance"
	"github.com/stretchr/testify/require"
)

func TestDsAcceptanceStore(t *testing.T) {
	t.Run("roundtrip", func(t *testing.T) {
		store, err := acceptancestore.NewDsAcceptanceStore(datastore.NewMapDatastore())
		require.NoError(t, err)

		acc := acceptance.Acceptance{
			Space: testutil.RandomDID(t),
			Blob: acceptance.Blob{
				Digest: testutil.RandomMultihash(t),
				Size:   uint64(1 + rand.IntN(1000)),
			},
			ExecutedAt: uint64(time.Now().Unix()),
			Cause:      testutil.RandomCID(t),
		}

		err = store.Put(t.Context(), acc)
		require.NoError(t, err)

		allocs, err := store.List(t.Context(), acc.Blob.Digest)
		require.NoError(t, err)
		require.Len(t, allocs, 1)
		require.Equal(t, acc, allocs[0])
	})

	t.Run("multiple", func(t *testing.T) {
		store, err := acceptancestore.NewDsAcceptanceStore(datastore.NewMapDatastore())
		require.NoError(t, err)

		acc0 := acceptance.Acceptance{
			Space: testutil.RandomDID(t),
			Blob: acceptance.Blob{
				Digest: testutil.RandomMultihash(t),
				Size:   uint64(1 + rand.IntN(1000)),
			},
			ExecutedAt: uint64(time.Now().Unix()),
			Cause:      testutil.RandomCID(t),
		}

		acc1 := acceptance.Acceptance{
			Space:      testutil.RandomDID(t),
			Blob:       acc0.Blob,
			ExecutedAt: uint64(time.Now().Unix()),
			Cause:      testutil.RandomCID(t),
		}

		err = store.Put(t.Context(), acc0)
		require.NoError(t, err)
		err = store.Put(t.Context(), acc1)
		require.NoError(t, err)

		allocs, err := store.List(t.Context(), acc0.Blob.Digest)
		require.NoError(t, err)
		require.Len(t, allocs, 2)

		if acc0.Space.String() == allocs[0].Space.String() {
			require.Equal(t, []acceptance.Acceptance{acc0, acc1}, allocs)
		} else {
			require.Equal(t, []acceptance.Acceptance{acc1, acc0}, allocs)
		}
	})
}
