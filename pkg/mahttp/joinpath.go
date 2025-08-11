package mahttp

import (
	"net/url"

	"github.com/multiformats/go-multiaddr"
)

// JoinPath joins a HTTP path onto an existing multiaddr. If the multiaddr
// includes the "http-path" protocol already, it is joined to the end of the
// existing path. If the multiaddr does not contain the "http-path" protocol
// then it is appended to the multiaddr, with the value specified.
func JoinPath(addr multiaddr.Multiaddr, path string) (multiaddr.Multiaddr, error) {
	found := false
	var out multiaddr.Multiaddr
	for _, comp := range addr {
		if comp.Code() == multiaddr.P_HTTP_PATH {
			p, err := url.PathUnescape(comp.Value())
			if err != nil {
				return nil, err
			}
			u, err := url.Parse(p)
			if err != nil {
				return nil, err
			}
			p = u.JoinPath(path).Path
			c, err := multiaddr.NewComponent("http-path", url.PathEscape(p))
			if err != nil {
				return nil, err
			}
			comp = *c
			found = true
		}
		out = append(out, comp)
	}

	if !found {
		c, err := multiaddr.NewComponent("http-path", url.PathEscape(path))
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}

	return out, nil
}
