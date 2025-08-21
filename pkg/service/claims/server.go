package claims

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/labstack/echo/v4"

	"github.com/storacha/go-ucanto/core/car"
	echofx "github.com/storacha/piri/pkg/fx/echo"
	"github.com/storacha/piri/pkg/server/handler"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/claimstore"
)

var _ echofx.RouteRegistrar = (*Server)(nil)

type Server struct {
	claims claimstore.ClaimStore
}

func NewServer(claims claimstore.ClaimStore) (*Server, error) {
	return &Server{claims}, nil
}

func (srv *Server) RegisterRoutes(e *echo.Echo) {
	e.GET("/claim/:claim", NewHandler(srv.claims).ToEcho())
}

func NewHandler(claims claimstore.ClaimStore) handler.Func {
	return func(ctx handler.Context) error {
		r := ctx.Request()
		parts := strings.Split(r.URL.Path, "/")
		c, err := cid.Parse(parts[len(parts)-1])
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("invalid claim CID: %w", err))
		}

		dlg, err := claims.Get(r.Context(), cidlink.Link{Cid: c})
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, fmt.Errorf("not found: %s", c))
			}
			return fmt.Errorf("failed to get claim: %w", err)
		}

		return ctx.Stream(http.StatusOK, car.ContentType, dlg.Archive())
	}
}
