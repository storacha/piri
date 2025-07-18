package storage

import (
	"fmt"
	"io"
	"net/http"

	"github.com/storacha/go-ucanto/server"
	ucanhttp "github.com/storacha/go-ucanto/transport/http"

	"github.com/storacha/piri/internal/telemetry"
)

type Server struct {
	ucanServer server.ServerView
}

func NewServer(service Service, options ...server.Option) (*Server, error) {
	ucanSrv, err := NewUCANServer(service, options...)
	if err != nil {
		return nil, fmt.Errorf("creating UCAN server: %w", err)
	}

	return &Server{ucanSrv}, nil
}

func (srv *Server) Serve(mux *http.ServeMux) {
	mux.Handle("POST /", NewHandler(srv.ucanServer))
}

func NewHandler(server server.ServerView) http.Handler {
	handler := func(w http.ResponseWriter, r *http.Request) error {
		res, err := server.Request(r.Context(), ucanhttp.NewHTTPRequest(r.Body, r.Header))
		if err != nil {
			return telemetry.NewHTTPError(fmt.Errorf("handling UCAN request: %w", err), http.StatusInternalServerError)
		}

		for key, vals := range res.Headers() {
			for _, v := range vals {
				w.Header().Add(key, v)
			}
		}

		if res.Status() != 0 {
			w.WriteHeader(res.Status())
		}

		_, err = io.Copy(w, res.Body())
		if err != nil {
			return fmt.Errorf("sending UCAN response: %w", err)
		}

		return nil
	}

	return telemetry.NewErrorReportingHandler(handler)
}
