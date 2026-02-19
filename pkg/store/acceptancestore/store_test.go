package acceptancestore_test

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/stretchr/testify/require"

	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/acceptancestore"
	"github.com/storacha/piri/pkg/store/acceptancestore/acceptance"
)

func TestDsAcceptanceStore(t *testing.T) {
	t.Run("roundtrip", func(t *testing.T) {
		s := acceptancestore.NewDatastoreStore(datastore.NewMapDatastore())

		acc := acceptance.Acceptance{
			Space: testutil.RandomDID(t),
			Blob: acceptance.Blob{
				Digest: testutil.RandomMultihash(t),
				Size:   uint64(1 + rand.IntN(1000)),
			},
			ExecutedAt: uint64(time.Now().Unix()),
			Cause:      testutil.RandomCID(t),
		}

		err := s.Put(t.Context(), acc)
		require.NoError(t, err)

		got, err := s.Get(t.Context(), acc.Blob.Digest, acc.Space)
		require.NoError(t, err)
		require.Equal(t, acc, got)

		gotAny, err := s.GetAny(t.Context(), acc.Blob.Digest)
		require.NoError(t, err)
		require.Equal(t, acc, gotAny)

		exists, err := s.Exists(t.Context(), acc.Blob.Digest)
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("multiple", func(t *testing.T) {
		s := acceptancestore.NewDatastoreStore(datastore.NewMapDatastore())

		digest := testutil.RandomMultihash(t)

		acc0 := acceptance.Acceptance{
			Space: testutil.RandomDID(t),
			Blob: acceptance.Blob{
				Digest: digest,
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

		err := s.Put(t.Context(), acc0)
		require.NoError(t, err)
		err = s.Put(t.Context(), acc1)
		require.NoError(t, err)

		// Get specific acceptances by space
		got0, err := s.Get(t.Context(), digest, acc0.Space)
		require.NoError(t, err)
		require.Equal(t, acc0, got0)

		got1, err := s.Get(t.Context(), digest, acc1.Space)
		require.NoError(t, err)
		require.Equal(t, acc1, got1)

		// GetAny should return one of them
		gotAny, err := s.GetAny(t.Context(), digest)
		require.NoError(t, err)
		require.True(t, gotAny.Space.String() == acc0.Space.String() || gotAny.Space.String() == acc1.Space.String())

		// Exists should return true
		exists, err := s.Exists(t.Context(), digest)
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("not found", func(t *testing.T) {
		s := acceptancestore.NewDatastoreStore(datastore.NewMapDatastore())

		digest := testutil.RandomMultihash(t)
		space := testutil.RandomDID(t)

		_, err := s.Get(t.Context(), digest, space)
		require.ErrorIs(t, err, store.ErrNotFound)

		_, err = s.GetAny(t.Context(), digest)
		require.ErrorIs(t, err, store.ErrNotFound)

		exists, err := s.Exists(t.Context(), digest)
		require.NoError(t, err)
		require.False(t, exists)
	})
}
