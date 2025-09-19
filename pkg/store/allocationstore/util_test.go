package allocationstore_test

import (
	"testing"

	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/allocationstore/allocation"
	"github.com/stretchr/testify/require"
)

func TestSizer(t *testing.T) {
	t.Run("gets size from allocation", func(t *testing.T) {
		allocs := allocationstore.NewMapAllocationStore()
		sizer := allocationstore.NewBlobSizer(allocs)

		data := testutil.RandomBytes(t, 32)
		digest := testutil.MultihashFromBytes(t, data)

		err := allocs.Put(t.Context(), allocation.Allocation{
			Space: testutil.RandomDID(t),
			Blob: allocation.Blob{
				Digest: digest,
				Size:   uint64(len(data)),
			},
			Expires: 0,
			Cause:   testutil.RandomCID(t),
		})
		require.NoError(t, err)

		size, err := sizer.Size(t.Context(), digest)
		require.NoError(t, err)

		require.Equal(t, uint64(len(data)), size)
	})

	t.Run("returns error if no allocation exists", func(t *testing.T) {
		allocs := allocationstore.NewMapAllocationStore()
		sizer := allocationstore.NewBlobSizer(allocs)

		data := testutil.RandomBytes(t, 32)
		digest := testutil.MultihashFromBytes(t, data)

		_, err := sizer.Size(t.Context(), digest)
		require.ErrorIs(t, err, store.ErrNotFound)
	})
}
