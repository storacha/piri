package access

import (
	"fmt"
	"strings"
	"testing"

	"github.com/storacha/go-libstoracha/testutil"
	"github.com/stretchr/testify/require"
)

func TestPatternAccess(t *testing.T) {
	t.Run("gets URL", func(t *testing.T) {
		prefix := "http://localhost/blob/"
		access, err := NewPatternAccess(fmt.Sprintf("%s{blob}", prefix))
		require.NoError(t, err)

		url, err := access.GetDownloadURL(testutil.RandomMultihash(t))
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(url.String(), prefix))
	})

	t.Run("missing pattern", func(t *testing.T) {
		_, err := NewPatternAccess("http://localhost/blob")
		require.Error(t, err)
		require.Contains(t, err.Error(), "URL string does not contain required pattern")
	})

	t.Run("invalid url", func(t *testing.T) {
		access, err := NewPatternAccess("://localhost/{blob}")
		require.NoError(t, err)

		_, err = access.GetDownloadURL(testutil.RandomMultihash(t))
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing protocol scheme")
	})
}
