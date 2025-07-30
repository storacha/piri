package principalresolver_test

import (
	"testing"

	"github.com/storacha/go-ucanto/did"
	"github.com/stretchr/testify/require"

	"github.com/storacha/piri/pkg/principalresolver"
)

func TestPrincipalResolver(t *testing.T) {
	p0, err := did.Parse("did:web:example.com")
	require.NoError(t, err)
	r, err := did.Parse("did:key:z6MkghfetkhrBZwUupJrv8MmYDH1JhKCQCGj1trbaZPA3dAd")
	require.NoError(t, err)
	p1, err := did.Parse("did:web:example.org")
	require.NoError(t, err)

	pm := map[string]string{p0.String(): r.String()}
	ppr, err := principalresolver.NewMapResolver(pm)
	require.NoError(t, err)

	resolved, err := ppr.ResolveDIDKey(t.Context(), p0)
	require.NoError(t, err)
	require.Equal(t, r, resolved)

	// cannot resolve DID not in mapping
	_, err = ppr.ResolveDIDKey(t.Context(), p1)
	require.NotNil(t, err)
}
