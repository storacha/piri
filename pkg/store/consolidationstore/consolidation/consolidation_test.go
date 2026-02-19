package consolidation

import (
	"testing"

	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/result/ok"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/stretchr/testify/require"
)

func TestConsolidation(t *testing.T) {
	t.Run("encode/decode roundtrip", func(t *testing.T) {
		c := createTestConsolidation(t)

		encoded, err := Encode(c)
		require.NoError(t, err)
		require.NotEmpty(t, encoded)

		decoded, err := Decode(encoded)
		require.NoError(t, err)

		requireEqualConsolidation(t, c, decoded)
	})

	t.Run("ToIPLD/FromIPLD roundtrip", func(t *testing.T) {
		c := createTestConsolidation(t)

		node, err := c.ToIPLD()
		require.NoError(t, err)
		require.NotNil(t, node)

		decoded, err := FromIPLD(node)
		require.NoError(t, err)

		requireEqualConsolidation(t, c, decoded)
	})

	t.Run("codec roundtrip", func(t *testing.T) {
		c := createTestConsolidation(t)
		codec := Codec{}

		encoded, err := codec.Encode(c)
		require.NoError(t, err)

		decoded, err := codec.Decode(encoded)
		require.NoError(t, err)

		requireEqualConsolidation(t, c, decoded)
	})
}

func createTestConsolidation(t *testing.T) Consolidation {
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

	return Consolidation{
		TrackInvocation:          inv,
		ConsolidateInvocationCID: randomCID(t),
	}
}

func randomCID(t *testing.T) cid.Cid {
	t.Helper()
	link := testutil.RandomCID(t)
	return link.(cidlink.Link).Cid
}

func requireEqualConsolidation(t *testing.T, expected, actual Consolidation) {
	t.Helper()

	// Compare invocation links (the canonical identifier)
	require.Equal(t, expected.TrackInvocation.Link(), actual.TrackInvocation.Link())
	// Compare consolidate CIDs
	require.Equal(t, expected.ConsolidateInvocationCID, actual.ConsolidateInvocationCID)
}
