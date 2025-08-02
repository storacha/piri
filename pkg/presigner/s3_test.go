package presigner

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
	ed25519 "github.com/storacha/go-ucanto/principal/ed25519/signer"
	"github.com/stretchr/testify/require"

	"github.com/storacha/go-libstoracha/testutil"
)

func TestS3Signer(t *testing.T) {
	endpoint, err := url.Parse("http://localhost:3000")
	require.NoError(t, err)

	signer := testutil.RandomSigner(t)

	accessKeyID := signer.DID().String()
	secretAccessKey := testutil.Must(ed25519.Format(signer))(t)

	t.Run("sign and verify", func(t *testing.T) {
		reqSigner, err := NewS3RequestPresigner(accessKeyID, secretAccessKey, *endpoint, "data")
		require.NoError(t, err)

		url, headers, err := reqSigner.SignUploadURL(t.Context(), testutil.RandomMultihash(t), 138, 900)
		require.NoError(t, err)

		fmt.Println(url.String())
		fmt.Printf("%+v\n", headers)

		_, _, err = reqSigner.VerifyUploadURL(t.Context(), url, headers)
		require.NoError(t, err)
	})

	t.Run("invalid URL", func(t *testing.T) {
		reqSigner, err := NewS3RequestPresigner(accessKeyID, secretAccessKey, *endpoint, "")
		require.NoError(t, err)

		url, headers, err := reqSigner.SignUploadURL(t.Context(), testutil.RandomMultihash(t), 138, 900)
		require.NoError(t, err)

		// mess with the url
		url.Path += "/index.html"

		_, _, err = reqSigner.VerifyUploadURL(t.Context(), url, headers)
		require.Error(t, err)

		require.Equal(t, err.Error(), "signature verification failed")
	})

	t.Run("invalid header", func(t *testing.T) {
		reqSigner, err := NewS3RequestPresigner(accessKeyID, secretAccessKey, *endpoint, "")
		require.NoError(t, err)

		url, headers, err := reqSigner.SignUploadURL(t.Context(), testutil.RandomMultihash(t), 138, 900)
		require.NoError(t, err)

		// mess with the headers
		headers.Set("Content-Length", "10000")

		_, _, err = reqSigner.VerifyUploadURL(t.Context(), url, headers)
		require.Error(t, err)

		require.Equal(t, err.Error(), "signature verification failed")
	})

	t.Run("requires sha2-256 digest", func(t *testing.T) {
		reqSigner, err := NewS3RequestPresigner(accessKeyID, secretAccessKey, *endpoint, "data")
		require.NoError(t, err)

		data := testutil.RandomBytes(t, 32)
		digest, err := multihash.Sum(data, uint64(multicodec.Sha2_512), -1)
		require.NoError(t, err)

		_, _, err = reqSigner.SignUploadURL(t.Context(), digest, uint64(len(data)), 900)
		require.EqualError(t, err, fmt.Sprintf("unsupported digest: %d", multicodec.Sha2_512))
	})
}
