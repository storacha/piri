package client

import (
	"crypto/ed25519"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v4"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/stretchr/testify/require"
)

func TestCreateAuthBearerTokenFromID(t *testing.T) {
	signer := testutil.RandomSigner(t)

	token, err := createAuthBearerTokenFromID(signer)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(token, "Bearer "))

	parsed, err := jwt.Parse(strings.TrimPrefix(token, "Bearer "), func(token *jwt.Token) (interface{}, error) {
		require.Equal(t, jwt.SigningMethodEdDSA.Alg(), token.Method.Alg())
		return ed25519.PublicKey(signer.Verifier().Raw()), nil
	})
	require.NoError(t, err)
	require.True(t, parsed.Valid)

	claims, ok := parsed.Claims.(jwt.MapClaims)
	require.True(t, ok)
	require.Equal(t, "storacha", claims["service_name"])
}

func TestWithBearerFromSignerSetsHeader(t *testing.T) {
	signer := testutil.RandomSigner(t)
	client := &Client{}

	err := WithBearerFromSigner(signer)(client)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(client.authHeader, "Bearer "))
}
