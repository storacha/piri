package http

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
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/presigner"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
)

var log = logging.Logger("blob-server")

type Blob struct {
	BlobStore       blobstore.Blobstore
	PreSigner       presigner.RequestPresigner
	AllocationStore allocationstore.AllocationStore
}

type Params struct {
	fx.In
	BlobStore       blobstore.Blobstore
	PreSigner       presigner.RequestPresigner
	AllocationStore allocationstore.AllocationStore
}

func NewBlob(params Params) *Blob {
	return &Blob{
		BlobStore:       params.BlobStore,
		PreSigner:       params.PreSigner,
		AllocationStore: params.AllocationStore,
	}
}

func (s *Blob) RegisterRoutes(e *echo.Echo) {
	// Create a group for blob routes
	group := e.Group("/blob")
	group.GET("/:blob", s.GetBlob)
	group.PUT("/:blob", s.PutBlob)
}

func (s *Blob) GetBlob(c echo.Context) error {
	if fsblobs, ok := s.BlobStore.(blobstore.FileSystemer); ok {
		// Strip the "/blob" prefix from the path for the file server
		req := c.Request()
		req.URL.Path = req.URL.Path[len("/blob"):]
		http.FileServer(fsblobs.FileSystem()).ServeHTTP(c.Response(), req)
		return nil
	}

	log.Error("blobstore does not support filesystem access")
	return echo.NewHTTPError(http.StatusInternalServerError, "not supported")
}

func (s *Blob) PutBlob(c echo.Context) error {
	req := c.Request()

	// Verify the upload URL and headers
	_, sHeaders, err := s.PreSigner.VerifyUploadURL(req.Context(), *req.URL, req.Header)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	}

	// Extract and decode the blob digest from the URL
	blobParam := c.Param("blob")
	_, bytes, err := multibase.Decode(blobParam)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("decoding multibase encoded digest: %v", err))
	}

	digest, err := multihash.Cast(bytes)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid multihash digest: %v", err))
	}

	// Check allocations
	results, err := s.AllocationStore.List(req.Context(), digest)
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

	// Get content length from signed headers
	contentLength, err := strconv.ParseInt(sHeaders.Get("Content-Length"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("parsing signed Content-Length header: %v", err))
	}

	// Store the blob
	err = s.BlobStore.Put(req.Context(), digest, uint64(contentLength), req.Body)
	if err != nil {
		log.Errorf("writing to: z%s: %v", digest.B58String(), err)
		if errors.Is(err, blobstore.ErrDataInconsistent) {
			return echo.NewHTTPError(http.StatusConflict, "data consistency check failed")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("write failed: %v", err))
	}

	return c.NoContent(http.StatusOK)
}
