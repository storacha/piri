package retrieval

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/server/retrieval"
	ucanhttp "github.com/storacha/go-ucanto/transport/http"
)

type Server struct {
	server server.ServerView[retrieval.Service]
}

func NewServer(service Service, options ...retrieval.Option) (*Server, error) {
	retrievalSrv, err := NewUCANServer(service, options...)
	if err != nil {
		return nil, fmt.Errorf("creating UCAN retrieval server: %w", err)
	}
	return &Server{retrievalSrv}, nil
}

func (srv *Server) RegisterRoutes(e *echo.Echo) {
	e.GET("/piece/:cid", NewHandler(srv.server))
}

func NewHandler(server server.ServerView[retrieval.Service]) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		r := ctx.Request()
		res, err := server.Request(r.Context(), ucanhttp.NewInboundRequest(r.URL, r.Body, r.Header))
		if err != nil {
			return fmt.Errorf("handling UCAN retrieval request: %w", err)
		}

		for key, vals := range res.Headers() {
			for _, v := range vals {
				ctx.Response().Header().Add(key, v)
			}
		}

		// content type is empty as it will have been set by ucanto transport codec
		return ctx.Stream(res.Status(), "", res.Body())
	}
}
