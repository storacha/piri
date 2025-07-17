package claims

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/labstack/echo/v4"

	"github.com/storacha/piri/internal/telemetry"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/claimstore"
)

type Server struct {
	claims claimstore.ClaimStore
}

func NewServer(claims claimstore.ClaimStore) (*Server, error) {
	return &Server{claims}, nil
}

func (srv *Server) RegisterRoutes(e *echo.Echo) {
	e.GET("/claim/:claim", echo.WrapHandler(NewHandler(srv.claims)))
}

func NewHandler(claims claimstore.ClaimStore) http.Handler {
	handler := func(w http.ResponseWriter, r *http.Request) error {
		parts := strings.Split(r.URL.Path, "/")
		c, err := cid.Parse(parts[len(parts)-1])
		if err != nil {
			return telemetry.NewHTTPError(fmt.Errorf("invalid claim CID: %w", err), http.StatusBadRequest)
		}

		dlg, err := claims.Get(r.Context(), cidlink.Link{Cid: c})
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return telemetry.NewHTTPError(fmt.Errorf("not found: %s", c), http.StatusNotFound)
			}

			return telemetry.NewHTTPError(fmt.Errorf("failed to get claim: %w", err), http.StatusInternalServerError)
		}

		_, err = io.Copy(w, dlg.Archive())
		if err != nil {
			return fmt.Errorf("serving claim: %s: %w", c, err)
		}

		return nil
	}

	return telemetry.NewErrorReportingHandler(handler)
}
