package mahttp

import (
	"net/url"
	"testing"

	"github.com/ipni/go-libipni/maurl"
	"github.com/multiformats/go-multiaddr"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/stretchr/testify/require"
)

func url2ma(t *testing.T, u string) multiaddr.Multiaddr {
	return testutil.Must(maurl.FromURL(testutil.Must(url.Parse(u))(t)))(t)
}

func TestJoinPath(t *testing.T) {
	cases := []struct {
		name     string
		addr     multiaddr.Multiaddr
		path     string
		expected string
	}{
		{
			name:     "from URL no trailing slash",
			addr:     url2ma(t, "https://example.org"),
			path:     "claim/{claim}",
			expected: "/dns/example.org/https/http-path/claim%2F%7Bclaim%7D",
		},
		{
			name:     "from URL with trailing slash",
			addr:     url2ma(t, "https://example.org/"),
			path:     "claim/{claim}",
			expected: "/dns/example.org/https/http-path/%2Fclaim%2F%7Bclaim%7D",
		},
		{
			name:     "from URL with existing path no trailing slash",
			addr:     url2ma(t, "https://example.org/foo"),
			path:     "claim/{claim}",
			expected: "/dns/example.org/https/http-path/%2Ffoo%2Fclaim%2F%7Bclaim%7D",
		},
		{
			name:     "from URL with existing path with trailing slash",
			addr:     url2ma(t, "https://example.org/foo/"),
			path:     "claim/{claim}",
			expected: "/dns/example.org/https/http-path/%2Ffoo%2Fclaim%2F%7Bclaim%7D",
		},
		{
			name:     "path with leading slash no existing http-path in multiaddr",
			addr:     testutil.Must(multiaddr.NewMultiaddr("/dns/example.org/https"))(t),
			path:     "/claim/{claim}",
			expected: "/dns/example.org/https/http-path/%2Fclaim%2F%7Bclaim%7D",
		},
		{
			name:     "path with leading slash existing http-path in multiaddr",
			addr:     testutil.Must(multiaddr.NewMultiaddr("/dns/example.org/https/http-path/foo"))(t),
			path:     "/claim/{claim}",
			expected: "/dns/example.org/https/http-path/foo%2Fclaim%2F%7Bclaim%7D",
		},
		{
			name:     "escaped characters in existing http-path in multiaddr",
			addr:     testutil.Must(multiaddr.NewMultiaddr("/dns/example.org/https/http-path/foo%24bar"))(t),
			path:     "claim/{claim}",
			expected: "/dns/example.org/https/http-path/foo%24bar%2Fclaim%2F%7Bclaim%7D",
		},
		{
			name:     "non http-path suffix",
			addr:     testutil.Must(multiaddr.NewMultiaddr("/dns/example.org/https/http-path/foo/p2p/12D3KooWMY8tm1dRbHNWwu3ZPrsKqw1Z96Kc16qDL9MTz7Brocz2"))(t),
			path:     "claim/{claim}",
			expected: "/dns/example.org/https/http-path/foo%2Fclaim%2F%7Bclaim%7D/p2p/12D3KooWMY8tm1dRbHNWwu3ZPrsKqw1Z96Kc16qDL9MTz7Brocz2",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			joined, err := JoinPath(testCase.addr, testCase.path)
			require.NoError(t, err)

			// multiaddrs should be valid URLs!
			u, err := maurl.ToURL(joined)
			require.NoError(t, err)
			t.Log(u.String())

			require.Equal(t, testCase.expected, joined.String())
		})
	}
}
