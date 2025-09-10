package server

import (
	"fmt"
	"net/http"

	"github.com/ipfs/go-cid"
	"github.com/labstack/echo/v4"
)

const piecePrefix = "/piece/"

// TODO support range requests
func (p *PDPHandler) handleDownloadByPieceCid(c echo.Context) error {
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

	// tells client this is a piece, aka custom MIME type for Filecoin pieces
	// TODO unsure if this is needed, but curio does it
	c.Response().Header().Set("Content-Type", "application/piece")
	// set Cache-Control settings
	// public: Can be cached by browsers AND intermediate caches (CDNs, proxies)
	// max-age=29030400: Cache for ~11 months (336 days)
	// immutable: This content will NEVER change - browser won't even check for updates
	c.Response().Header().Set("Cache-Control", "public, max-age=29030400, immutable")
	// set Entity tag - a unique identifier for this specific version of the content
	// Since we're using the CIDs, this is perfect for immutable content
	c.Response().Header().Set("Etag", fmt.Sprintf(`"%s"`, pieceCid.String()))
	// set Vary which tells caches that if we ever add compression support in the future (e.g. gzip), the cached version
	// should vary based on what encoding the client accepts
	c.Response().Header().Set("Vary", "Accept-Encoding")
	// how big these bytes be
	c.Response().Header().Set("Content-Length", fmt.Sprintf("%d", obj.Size))

	if c.Request().Method == http.MethodHead {
		// TODO, in the above call to ReadPiece we read the entire piece into memory, which is
		// inefficient, rather we should get it's size, which requires a blobstore supporting proper readers
		return c.NoContent(http.StatusOK)
	}

	return c.Stream(http.StatusOK, "application/piece", obj.Data)

}
