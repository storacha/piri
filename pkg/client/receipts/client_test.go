package receipts_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/message"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/core/receipt/ran"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/result/failure"
	"github.com/storacha/go-ucanto/core/result/ok"
	"github.com/storacha/go-ucanto/transport/car/response"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/storacha/piri/pkg/client/receipts"
	"github.com/stretchr/testify/require"
)

func TestFetch(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		inv, err := invocation.Invoke(
			testutil.Alice,
			testutil.Service,
			ucan.NewCapability(
				"test/receipt",
				testutil.Alice.DID().String(),
				ucan.NoCaveats{},
			),
		)
		require.NoError(t, err)

		rcpt, err := receipt.Issue(
			testutil.Alice,
			result.Ok[ok.Unit, failure.IPLDBuilderFailure](ok.Unit{}),
			ran.FromInvocation(inv),
		)
		require.NoError(t, err)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			msg, err := message.Build(nil, []receipt.AnyReceipt{rcpt})
			require.NoError(t, err)
			res, err := response.Encode(msg)
			require.NoError(t, err)
			_, err = io.Copy(w, res.Body())
			require.NoError(t, err)
		}))
		defer server.Close()

		endpoint, err := url.Parse(server.URL)
		require.NoError(t, err)

		client := receipts.NewClient(endpoint)
		result, err := client.Fetch(t.Context(), inv.Link())
		require.NoError(t, err)
		require.Equal(t, inv.Link(), result.Ran().Link())
	})

	t.Run("not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		endpoint, err := url.Parse(server.URL)
		require.NoError(t, err)

		client := receipts.NewClient(endpoint)
		_, err = client.Fetch(t.Context(), testutil.RandomCID(t))
		require.ErrorIs(t, err, receipts.ErrNotFound)
	})

	t.Run("error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		endpoint, err := url.Parse(server.URL)
		require.NoError(t, err)

		client := receipts.NewClient(endpoint)
		_, err = client.Fetch(t.Context(), testutil.RandomCID(t))
		require.Error(t, err)
		require.ErrorContains(t, err, "500")
	})
}
