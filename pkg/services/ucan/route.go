package ucan

import (
	"fmt"
	"io"
	"net/http"

	logging "github.com/ipfs/go-log"
	"github.com/labstack/echo/v4"
	ucanhttp "github.com/storacha/go-ucanto/transport/http"
	"go.uber.org/fx"
)

var log = logging.Logger("storage-server")

type Router struct {
	ucanServer *Server
}

type RouterParams struct {
	fx.In
	UCANServer *Server
}

func NewRouter(p RouterParams) *Router {
	return &Router{
		ucanServer: p.UCANServer,
	}
}

func (s *Router) RegisterRoutes(e *echo.Echo) {
	log.Info("registering storage server routes")
	e.POST("/", echo.WrapHandler(s.ucanHandler()))
}

func (s *Router) ucanHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Debugf("handling UCAN request: %s %s", r.Method, r.URL.Path)

		// Parse the UCAN HTTP request
		res, err := s.ucanServer.Request(ucanhttp.NewHTTPRequest(r.Body, r.Header))
		if err != nil {
			log.Errorf("failed to parse UCAN request: %v", err)
			http.Error(w, "failed to parse request", http.StatusBadRequest)
			return
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
			http.Error(w, fmt.Errorf("sending UCAN response: %w", err).Error(), http.StatusInternalServerError)
		}

		return
	})
}
