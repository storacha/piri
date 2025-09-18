package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/server"
	ucanretrieval "github.com/storacha/go-ucanto/server/retrieval"

	"github.com/storacha/piri/pkg/build"
	"github.com/storacha/piri/pkg/service/blobs"
	"github.com/storacha/piri/pkg/service/claims"
	"github.com/storacha/piri/pkg/service/publisher"
	"github.com/storacha/piri/pkg/service/retrieval"
	"github.com/storacha/piri/pkg/service/storage"
)

var log = logging.Logger("server")

type serverConfig struct {
	ucanSrvOpts          []server.Option
	ucanRetrievalSrvOpts []ucanretrieval.Option
}

type Option = func(c *serverConfig)

func WithUCANServerOptions(options ...server.Option) Option {
	return func(c *serverConfig) {
		c.ucanSrvOpts = options
	}
}

func WithUCANRetrievalServerOptions(options ...ucanretrieval.Option) Option {
	return func(c *serverConfig) {
		c.ucanRetrievalSrvOpts = options
	}
}

// ListenAndServe creates a new storage node HTTP server, and starts it up.
func ListenAndServe(addr string, storageSvc storage.Service, retrievalSvc retrieval.Service, options ...Option) error {
	srvMux, err := NewServer(storageSvc, retrievalSvc, options...)
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
func NewServer(storageSvc storage.Service, retrievalSvc retrieval.Service, options ...Option) (*echo.Echo, error) {
	cfg := serverConfig{}
	for _, opt := range options {
		opt(&cfg)
	}

	mux := echo.New()
	mux.GET("/", echo.WrapHandler(NewHandler(storageSvc.ID())))

	httpUcanSrv, err := storage.NewServer(storageSvc, cfg.ucanSrvOpts...)
	if err != nil {
		return nil, fmt.Errorf("creating UCAN server: %w", err)
	}
	httpUcanSrv.RegisterRoutes(mux)

	httpUcanRetrievalSrv, err := retrieval.NewServer(retrievalSvc, cfg.ucanRetrievalSrvOpts...)
	if err != nil {
		return nil, fmt.Errorf("creating UCAN retrieval server: %w", err)
	}
	httpUcanRetrievalSrv.RegisterRoutes(mux)

	httpClaimsSrv, err := claims.NewServer(storageSvc.Claims().Store())
	if err != nil {
		return nil, fmt.Errorf("creating claims server: %w", err)
	}
	httpClaimsSrv.RegisterRoutes(mux)

	if storageSvc.PDP() == nil {
		httpBlobsSrv, err := blobs.NewServer(storageSvc.Blobs().Presigner(), storageSvc.Blobs().Allocations(), storageSvc.Blobs().Store())
		if err != nil {
			return nil, fmt.Errorf("creating blobs server: %w", err)
		}
		httpBlobsSrv.RegisterRoutes(mux)
	}

	publisherStore := storageSvc.Claims().Publisher().Store()
	encodableStore, ok := publisherStore.(store.EncodeableStore)
	if !ok {
		return nil, errors.New("publisher store does not implement EncodableStore")
	}

	httpPublisherSrv, err := publisher.NewServer(encodableStore)
	if err != nil {
		return nil, fmt.Errorf("creating IPNI publisher server: %w", err)
	}
	httpPublisherSrv.RegisterRoutes(mux)

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
			w.Header().Set("Content-Type", "application/json")
			data, err := json.Marshal(&info)
			if err != nil {
				log.Errorf("failed JSON marshal server info: %w", err)
				http.Error(w, "failed JSON marshal server info", http.StatusInternalServerError)
				return
			}
			w.Write(data)
		} else {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write([]byte(fmt.Sprintf("🔥 piri %s\n", info.Build.Version)))
			w.Write([]byte("- https://github.com/storacha/piri\n"))
			w.Write([]byte(fmt.Sprintf("- %s", info.ID)))
		}
	})
}
