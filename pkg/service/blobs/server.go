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

	"github.com/storacha/piri/internal/telemetry"
	"github.com/storacha/piri/pkg/presigner"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
)

var log = logging.Logger("blobs")

type Server struct {
	blobs     blobstore.Blobstore
	presigner presigner.RequestPresigner
	allocs    allocationstore.AllocationStore
}

func NewServer(presigner presigner.RequestPresigner, allocs allocationstore.AllocationStore, blobs blobstore.Blobstore) (*Server, error) {
	return &Server{blobs, presigner, allocs}, nil
}

func (srv *Server) RegisterRoutes(e *echo.Echo) {
	e.GET("/blob/:blob", echo.WrapHandler(NewBlobGetHandler(srv.blobs)))
	e.PUT("/blob/:blob", echo.WrapHandler(NewBlobPutHandler(srv.presigner, srv.allocs, srv.blobs)))
}

func NewBlobGetHandler(blobs blobstore.Blobstore) http.Handler {
	if fsblobs, ok := blobs.(blobstore.FileSystemer); ok {
		serveHTTP := http.FileServer(fsblobs.FileSystem()).ServeHTTP
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.URL.Path = r.URL.Path[len("/blob"):]
			serveHTTP(w, r)
		})
	}

	log.Error("blobstore does not support filesystem access")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not supported", http.StatusInternalServerError)
	})
}

func NewBlobPutHandler(presigner presigner.RequestPresigner, allocs allocationstore.AllocationStore, blobs blobstore.Blobstore) http.Handler {
	handler := func(w http.ResponseWriter, r *http.Request) error {
		_, sHeaders, err := presigner.VerifyUploadURL(r.Context(), *r.URL, r.Header)
		if err != nil {
			return telemetry.NewHTTPError(err, http.StatusUnauthorized)
		}

		parts := strings.Split(r.URL.Path, "/")
		_, bytes, err := multibase.Decode(parts[len(parts)-1])
		if err != nil {
			return telemetry.NewHTTPError(fmt.Errorf("decoding multibase encoded digest: %w", err), http.StatusBadRequest)
		}

		digest, err := multihash.Cast(bytes)
		if err != nil {
			return telemetry.NewHTTPError(fmt.Errorf("invalid multihash digest: %w", err), http.StatusBadRequest)
		}

		results, err := allocs.List(r.Context(), digest)
		if err != nil {
			return telemetry.NewHTTPError(fmt.Errorf("list allocations failed: %w", err), http.StatusInternalServerError)
		}

		if len(results) == 0 {
			return telemetry.NewHTTPError(fmt.Errorf("missing allocation for write to: z%s", digest.B58String()), http.StatusForbidden)
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
			return telemetry.NewHTTPError(errors.New("expired allocation"), http.StatusForbidden)
		}

		log.Infof("Found %d allocations for write to: z%s", len(results), digest.B58String())

		// ensure the size comes from a signed header
		contentLength, err := strconv.ParseInt(sHeaders.Get("Content-Length"), 10, 64)
		if err != nil {
			return telemetry.NewHTTPError(fmt.Errorf("parsing signed Content-Length header: %w", err), http.StatusInternalServerError)
		}

		err = blobs.Put(r.Context(), digest, uint64(contentLength), r.Body)
		if err != nil {
			log.Errorf("writing to: z%s: %w", digest.B58String(), err)
			if errors.Is(err, blobstore.ErrDataInconsistent) {
				return telemetry.NewHTTPError(errors.New("data consistency check failed"), http.StatusConflict)
			}

			return telemetry.NewHTTPError(fmt.Errorf("write failed: %w", err), http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusOK)
		return nil
	}

	return telemetry.NewErrorReportingHandler(handler)
}
