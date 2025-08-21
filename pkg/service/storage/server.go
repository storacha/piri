package storage

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/storacha/go-ucanto/server"
	ucanhttp "github.com/storacha/go-ucanto/transport/http"
	"github.com/storacha/piri/pkg/server/handler"
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

func (srv *Server) RegisterRoutes(e *echo.Echo) {
	e.POST("/", NewHandler(srv.ucanServer).ToEcho())
}

func NewHandler(server server.ServerView) handler.Func {
	return func(ctx handler.Context) error {
		r := ctx.Request()
		res, err := server.Request(r.Context(), ucanhttp.NewHTTPRequest(r.Body, r.Header))
		if err != nil {
			return fmt.Errorf("handling UCAN request: %w", err)
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
