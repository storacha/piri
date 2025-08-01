package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
)

func setHeaders(w http.ResponseWriter, pieceCid cid.Cid) {
	w.Header().Set("Vary", "Accept-Encoding")
	etag := `"` + pieceCid.String() + `.gz"` // must be quoted
	w.Header().Set("Etag", etag)
	w.Header().Set("Content-Type", "application/piece")
	w.Header().Set("Cache-Control", "public, max-age=29030400, immutable")
}

// For data served by the endpoints in the HTTP server that never changes
// (eg pieces identified by a piece CID) send a cache header with a constant,
// non-zero last modified time.
var lastModified = time.UnixMilli(1)

// TODO: since the blobstore interface doesn't return a read seeker, we make one, this won't work long term
// and requires changes to the interface, or a new one.
func makeReadSeeker(r io.Reader) (io.ReadSeeker, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func serveContent(res http.ResponseWriter, req *http.Request, size abi.UnpaddedPieceSize, content io.ReadSeeker) {
	// Note that the last modified time is a constant value because the data
	// in a piece identified by a cid will never change.

	if req.Method == http.MethodHead {
		// For an HTTP HEAD request ServeContent doesn't send any data (just headers)
		http.ServeContent(res, req, "", time.Time{}, nil)
		return
	}

	// Send the content
	res.Header().Set("Content-Length", fmt.Sprintf("%d", size))
	http.ServeContent(res, req, "", lastModified, content)
}
