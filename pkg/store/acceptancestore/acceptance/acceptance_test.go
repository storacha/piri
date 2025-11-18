package acceptance_test

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/ipld/go-ipld-prime/codec/dagjson"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/piri/pkg/store/acceptancestore/acceptance"
	"github.com/stretchr/testify/require"
)

func TestRoundtrip(t *testing.T) {
	t.Run("without PDP", func(t *testing.T) {
		a := acceptance.Acceptance{
			Space: testutil.RandomDID(t),
			Blob: acceptance.Blob{
				Digest: testutil.RandomMultihash(t),
				Size:   rand.Uint64N(1000000),
			},
			ExecutedAt: uint64(time.Now().Unix()),
			Cause:      testutil.RandomCID(t),
		}

		buf, err := acceptance.Encode(a, dagjson.Encode)
		require.NoError(t, err)
		t.Log(string(buf))

		a2, err := acceptance.Decode(buf, dagjson.Decode)
		require.NoError(t, err)
		require.Equal(t, a, a2)
	})

	t.Run("with PDP", func(t *testing.T) {
		a := acceptance.Acceptance{
			Space: testutil.RandomDID(t),
			Blob: acceptance.Blob{
				Digest: testutil.RandomMultihash(t),
				Size:   rand.Uint64N(1000000),
			},
			PDPAccept: &acceptance.Promise{
				UcanAwait: acceptance.Await{
					Selector: ".out.ok",
					Link:     testutil.RandomCID(t),
				},
			},
			ExecutedAt: uint64(time.Now().Unix()),
			Cause:      testutil.RandomCID(t),
		}

		buf, err := acceptance.Encode(a, dagjson.Encode)
		require.NoError(t, err)
		t.Log(string(buf))

		a2, err := acceptance.Decode(buf, dagjson.Decode)
		require.NoError(t, err)
		require.Equal(t, a, a2)
	})
}
