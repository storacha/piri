package blobs

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
	"github.com/multiformats/go-multibase"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/digestutil"

	echofx "github.com/storacha/piri/pkg/fx/echo"
	"github.com/storacha/piri/pkg/presigner"
	"github.com/storacha/piri/pkg/server/handler"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
)

var log = logging.Logger("blobs")

var _ echofx.RouteRegistrar = (*Server)(nil)

type Server struct {
	blobs     blobstore.Blobstore
	presigner presigner.RequestPresigner
	allocs    allocationstore.AllocationStore
}

func NewServer(presigner presigner.RequestPresigner, allocs allocationstore.AllocationStore, blobs blobstore.Blobstore) (*Server, error) {
	return &Server{blobs, presigner, allocs}, nil
}

func (srv *Server) RegisterRoutes(e *echo.Echo) {
	e.GET("/blob/:blob", NewBlobGetHandler(srv.blobs).ToEcho())
	e.PUT("/blob/:blob", NewBlobPutHandler(srv.presigner, srv.allocs, srv.blobs).ToEcho())
}

func NewBlobGetHandler(blobs blobstore.Blobstore) handler.Func {
	return func(ctx handler.Context) error {
		r, w := ctx.Request(), ctx.Response()

		// Parse digest from path (e.g., /blob/{digest})
		parts := strings.Split(r.URL.Path, "/")
		digestStr := parts[len(parts)-1]

		digest, err := digestutil.Parse(digestStr)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("invalid digest: %w", err))
		}

		obj, err := blobs.Get(r.Context(), digest)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, "blob not found")
			}
			return fmt.Errorf("getting blob: %w", err)
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.FormatInt(obj.Size(), 10))
		w.WriteHeader(http.StatusOK)

		body := obj.Body()
		defer body.Close()

		_, err = io.Copy(w, body)
		if err != nil {
			log.Errorf("streaming blob z%s: %v", digest.B58String(), err)
			return nil // Already started writing, can't change status code
		}

		return nil
	}
}

func NewBlobPutHandler(presigner presigner.RequestPresigner, allocs allocationstore.AllocationStore, blobs blobstore.Blobstore) handler.Func {
	return func(ctx handler.Context) error {
		r, w := ctx.Request(), ctx.Response()
		_, sHeaders, err := presigner.VerifyUploadURL(r.Context(), *r.URL, r.Header)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, err)
		}

		parts := strings.Split(r.URL.Path, "/")
		_, bytes, err := multibase.Decode(parts[len(parts)-1])
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("decoding multibase encoded digest: %w", err))
		}

		digest, err := multihash.Cast(bytes)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("invalid multihash digest: %w", err))
		}

		alloc, err := allocs.GetAny(r.Context(), digest)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return echo.NewHTTPError(http.StatusForbidden, fmt.Errorf("missing allocation for write to: z%s", digest.B58String()))
			}
			return fmt.Errorf("getting allocation: %w", err)
		}

		if alloc.Expires <= uint64(time.Now().Unix()) {
			return echo.NewHTTPError(http.StatusForbidden, "expired allocation")
		}

		log.Infof("Found allocation for write to: z%s", digest.B58String())

		// ensure the size comes from a signed header
		contentLength, err := strconv.ParseInt(sHeaders.Get("Content-Length"), 10, 64)
		if err != nil {
			return fmt.Errorf("parsing signed Content-Length header: %w", err)
		}

		err = blobs.Put(r.Context(), digest, uint64(contentLength), r.Body)
		if err != nil {
			log.Errorf("writing to: z%s: %w", digest.B58String(), err)
			if errors.Is(err, blobstore.ErrDataInconsistent) {
				return echo.NewHTTPError(http.StatusConflict, "data consistency check failed")
			}

			return fmt.Errorf("write failed: %w", err)
		}

		w.WriteHeader(http.StatusOK)
		return nil
	}
}
