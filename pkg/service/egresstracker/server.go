package egresstracker

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/ipfs/go-cid"
	"github.com/labstack/echo/v4"

	"github.com/storacha/go-ucanto/core/car"
	echofx "github.com/storacha/piri/pkg/fx/echo"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/retrievaljournal"
)

var _ echofx.RouteRegistrar = (*Server)(nil)

const ReceiptsPath = "/receipts"

type Server struct {
	egressBatches retrievaljournal.Journal
}

func NewServer(egressBatches retrievaljournal.Journal) (*Server, error) {
	return &Server{egressBatches}, nil
}

func (srv *Server) RegisterRoutes(e *echo.Echo) {
	e.GET(ReceiptsPath+"/:cid", NewHandler(srv.egressBatches))
}

func NewHandler(egressBatches retrievaljournal.Journal) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		cid, err := cid.Parse(ctx.Param("cid"))
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("invalid batch CID: %w", err))
		}

		batch, err := egressBatches.GetBatch(ctx.Request().Context(), cid)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, fmt.Errorf("batch not found: %s", cid))
			}

			return fmt.Errorf("failed to get batch from store: %w", err)
		}
		defer batch.Close()

		return ctx.Stream(http.StatusOK, car.ContentType, batch)
	}
}
