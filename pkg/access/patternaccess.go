package access

import (
	"fmt"
	"net/url"
	"strings"

	multihash "github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/digestutil"
)

const pattern = "{blob}"

type PatternAccess struct {
	urlPattern string
}

// GetDownloadURL implements Access.
func (p *PatternAccess) GetDownloadURL(digest multihash.Multihash) (url.URL, error) {
	u, err := url.ParseRequestURI(strings.ReplaceAll(p.urlPattern, pattern, digestutil.Format(digest)))
	if err != nil {
		return url.URL{}, err
	}
	return *u, nil
}

var _ Access = (*PatternAccess)(nil)

// NewPatternAccess creates a new [Access] instance for accessing blobs where
// the URL is created from a string that contains the placeholder pattern:
// "{blob}".
//
// e.g. "http://localhost:3000/blob/{blob}"
func NewPatternAccess(urlPattern string) (*PatternAccess, error) {
	if !strings.Contains(urlPattern, pattern) {
		return nil, fmt.Errorf(`URL string does not contain required pattern: "%s"`, pattern)
	}
	return &PatternAccess{urlPattern}, nil
}
