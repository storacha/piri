package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/labstack/echo/v4"
)

const piecePrefix = "/piece/"

func (p *PDP) handleDownloadByPieceCid(c echo.Context) error {
	ctx := c.Request().Context()

	// Remove the path up to the piece cid
	prefixLen := len(piecePrefix)
	if len(c.Request().URL.Path) <= prefixLen {
		errMsg := fmt.Sprintf("path %s is missing piece CID", c.Request().URL.Path)
		log.Error(errMsg)
		return c.String(http.StatusBadRequest, errMsg)
	}

	pieceCidStr := c.Request().URL.Path[prefixLen:]
	pieceCid, err := cid.Parse(pieceCidStr)
	if err != nil {
		errMsg := fmt.Sprintf("parsing piece CID '%s': %s", pieceCidStr, err.Error())
		log.Error(errMsg)
		return c.String(http.StatusBadRequest, errMsg)
	}

	// Get a reader over the piece
	// TODO we will want to wait on the PieceStore task to complete before allowing this read to go through,
	// else the piece may not exist. Alternately, we could query it from the stash via a lookup of parked_pice_ref joinned on another table.
	obj, err := p.Service.ReadPiece(ctx, pieceCid)
	if err != nil {
		errMsg := fmt.Sprintf("server error getting content for piece CID %s: %s", pieceCid, err)
		log.Error(errMsg)
		return err

	}

	bodyReadSeeker, err := makeReadSeeker(obj.Data)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	setHeaders(c.Response(), pieceCid)
	serveContent(c.Response(), c.Request(), abi.UnpaddedPieceSize(obj.Size), bodyReadSeeker)
	return nil
}

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
