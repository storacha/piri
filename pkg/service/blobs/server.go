package blobs

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
	"github.com/multiformats/go-multibase"
	"github.com/multiformats/go-multihash"

	echofx "github.com/storacha/piri/pkg/fx/echo"
	"github.com/storacha/piri/pkg/presigner"
	"github.com/storacha/piri/pkg/server/handler"
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
	if fsblobs, ok := blobs.(blobstore.FileSystemer); ok {
		serveHTTP := http.FileServer(fsblobs.FileSystem()).ServeHTTP
		return func(ctx handler.Context) error {
			r, w := ctx.Request(), ctx.Response()
			r.URL.Path = r.URL.Path[len("/blob"):]
			serveHTTP(w, r)
			return nil
		}
	}

	log.Error("blobstore does not support filesystem access")
	return func(ctx handler.Context) error {
		return echo.ErrMethodNotAllowed
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

		results, err := allocs.List(r.Context(), digest)
		if err != nil {
			return fmt.Errorf("listing allocations: %w", err)
		}

		if len(results) == 0 {
			return echo.NewHTTPError(http.StatusForbidden, fmt.Errorf("missing allocation for write to: z%s", digest.B58String()))
		}

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
