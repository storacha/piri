package delegationstore

import (
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/result/ok"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/stretchr/testify/require"
)

func TestDsDelegationStore(t *testing.T) {
	t.Run("roundtrip", func(t *testing.T) {
		store, err := NewDsDelegationStore(datastore.NewMapDatastore())
		require.NoError(t, err)

		dlg, err := delegation.Delegate(
			testutil.RandomSigner(t),
			testutil.RandomDID(t),
			[]ucan.Capability[ok.Unit]{
				ucan.NewCapability("test/test", testutil.RandomDID(t).String(), ok.Unit{}),
			},
		)
		require.NoError(t, err)

		err = store.Put(t.Context(), dlg)
		require.NoError(t, err)

		res, err := store.Get(t.Context(), dlg.Link())
		require.NoError(t, err)
		testutil.RequireEqualDelegation(t, dlg, res)
	})
}
