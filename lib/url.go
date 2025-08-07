package lib

import (
	"fmt"
	"net/url"
	"strings"
)

func ParseAndNormalizeURL(in string) (*url.URL, error) {
	if in == "" {
		return nil, fmt.Errorf("cannot parse and normalize empty url")
	}

	u, err := url.Parse(in)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %w", err)
	}
	u.Path = strings.TrimRight(u.Path, "/")

	return u, nil
}
