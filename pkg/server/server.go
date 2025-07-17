package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/piri/pkg/build"
	"github.com/storacha/piri/pkg/service/blobs"
	"github.com/storacha/piri/pkg/service/claims"
	"github.com/storacha/piri/pkg/service/publisher"
	"github.com/storacha/piri/pkg/service/storage"
)

var log = logging.Logger("server")

// ListenAndServe creates a new storage node HTTP server, and starts it up.
func ListenAndServe(addr string, service storage.Service, options ...server.Option) error {
	srvMux, err := NewServer(service, options...)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:    addr,
		Handler: srvMux,
	}
	log.Infof("Listening on %s", addr)
	err = srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// NewServer creates a new storage node server.
func NewServer(service storage.Service, options ...server.Option) (*http.ServeMux, error) {
	mux := http.NewServeMux()
	mux.Handle("GET /{$}", NewHandler(service.ID()))

	httpUcanSrv, err := storage.NewServer(service, options...)
	if err != nil {
		return nil, fmt.Errorf("creating UCAN server: %w", err)
	}
	httpUcanSrv.Serve(mux)

	httpClaimsSrv, err := claims.NewServer(service.Claims().Store())
	if err != nil {
		return nil, fmt.Errorf("creating claims server: %w", err)
	}
	httpClaimsSrv.Serve(mux)

	if service.PDP() == nil {
		httpBlobsSrv, err := blobs.NewServer(service.Blobs().Presigner(), service.Blobs().Allocations(), service.Blobs().Store())
		if err != nil {
			return nil, fmt.Errorf("creating blobs server: %w", err)
		}
		httpBlobsSrv.Serve(mux)
	}

	publisherStore := service.Claims().Publisher().Store()
	encodableStore, ok := publisherStore.(store.EncodeableStore)
	if !ok {
		return nil, errors.New("publisher store does not implement EncodableStore")
	}

	httpPublisherSrv, err := publisher.NewServer(encodableStore)
	if err != nil {
		return nil, fmt.Errorf("creating IPNI publisher server: %w", err)
	}
	httpPublisherSrv.Serve(mux)

	return mux, nil
}

type ServerInfo struct {
	ID    string    `json:"id"`
	Build BuildInfo `json:"build"`
}

type BuildInfo struct {
	Version string `json:"version"`
	Repo    string `json:"repo"`
}

// NewHandler displays version info.
func NewHandler(id principal.Signer) http.Handler {
	info := ServerInfo{
		ID: id.DID().String(),
		Build: BuildInfo{
			Version: build.Version,
			Repo:    "https://github.com/storacha/piri",
		},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Accept"), "application/json") {
			json, err := json.Marshal(&info)
			if err != nil {
				log.Errorf("failed JSON marshal server info: %w", err)
                                http.Error(w, "failed JSON marshal server info", http.StatusInternalServerError)
				return
			}
			w.Write(json)
		} else {
			w.Write([]byte(fmt.Sprintf("🔥 piri %s\n", info.Build.Version)))
			w.Write([]byte("- https://github.com/storacha/piri\n"))
			w.Write([]byte(fmt.Sprintf("- %s", info.ID)))
		}
	})
}
