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
