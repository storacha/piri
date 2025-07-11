package blobs

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
	"github.com/multiformats/go-multibase"
	"github.com/multiformats/go-multihash"

	"github.com/storacha/piri/pkg/presigner"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
)

var log = logging.Logger("fx/blobs")

// Handler wraps blobs handler functionality for Echo
type Handler struct {
	blobs     blobstore.Blobstore
	presigner presigner.RequestPresigner
	allocs    allocationstore.AllocationStore
}

// NewHandler creates a new blobs handler
func NewHandler(presigner presigner.RequestPresigner, allocs allocationstore.AllocationStore, blobs blobstore.Blobstore) (*Handler, error) {
	return &Handler{blobs, presigner, allocs}, nil
}

// RegisterRoutes registers the blobs routes with Echo
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.GET("/blob/:blob", h.handleGetBlob)
	e.PUT("/blob/:blob", h.handlePutBlob)
}

// handleGetBlob handles GET /blob/:blob requests
func (h *Handler) handleGetBlob(c echo.Context) error {
	if fsblobs, ok := h.blobs.(blobstore.FileSystemer); ok {
		// Serve directly from filesystem if supported
		fileServer := http.FileServer(fsblobs.FileSystem())
		// Strip the /blob prefix and serve the file
		c.Request().URL.Path = "/" + c.Param("blob")
		fileServer.ServeHTTP(c.Response(), c.Request())
		return nil
	}

	log.Error("blobstore does not support filesystem access")
	return echo.NewHTTPError(http.StatusInternalServerError, "not supported")
}

// handlePutBlob handles PUT /blob/:blob requests
func (h *Handler) handlePutBlob(c echo.Context) error {
	ctx := c.Request().Context()

	// Verify the upload URL
	_, sHeaders, err := h.presigner.VerifyUploadURL(ctx, *c.Request().URL, c.Request().Header)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	}

	blobParam := c.Param("blob")

	// Decode the multibase encoded digest
	_, bytes, err := multibase.Decode(blobParam)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("decoding multibase encoded digest: %v", err))
	}

	digest, err := multihash.Cast(bytes)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid multihash digest: %v", err))
	}

	// Check allocations
	results, err := h.allocs.List(ctx, digest)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("list allocations failed: %v", err))
	}

	if len(results) == 0 {
		return echo.NewHTTPError(http.StatusForbidden, fmt.Sprintf("missing allocation for write to: z%s", digest.B58String()))
	}

	// Check if any allocation is not expired
	expired := true
	for _, a := range results {
		exp := a.Expires
		if exp > uint64(time.Now().Unix()) {
			expired = false
			break
		}
	}

	if expired {
		return echo.NewHTTPError(http.StatusForbidden, "expired allocation")
	}

	log.Infof("Found %d allocations for write to: z%s", len(results), digest.B58String())

	// Ensure the size comes from a signed header
	contentLength, err := strconv.ParseInt(sHeaders.Get(echo.HeaderContentLength), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("parsing signed Content-Length header: %v", err))
	}

	// Store the blob
	err = h.blobs.Put(ctx, digest, uint64(contentLength), c.Request().Body)
	if err != nil {
		log.Errorf("writing to: z%s: %v", digest.B58String(), err)
		if errors.Is(err, blobstore.ErrDataInconsistent) {
			return echo.NewHTTPError(http.StatusConflict, "data consistency check failed")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("write failed: %v", err))
	}

	return c.NoContent(http.StatusOK)
}
