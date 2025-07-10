package ucan

import (
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
	"github.com/storacha/go-ucanto/principal"
	ucanserver "github.com/storacha/go-ucanto/server"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/fx/ucan/handlers"
	"github.com/storacha/piri/pkg/service/storage"
)

type Server struct {
	ucanServer ucanserver.ServerView
}

var Module = fx.Module("ucan/server",
	handlers.Module,
	fx.Provide(
		NewServer,
	),
)

var log = logging.Logger("ucan")

type Params struct {
	fx.In

	Echo    *echo.Echo
	ID      principal.Signer
	Options []ucanserver.Option `group:"ucan_options"`
}

func NewServer(p Params) (*Server, error) {
	ucanSvr, err := ucanserver.NewServer(p.ID, p.Options...)
	if err != nil {
		return nil, fmt.Errorf("creating ucan server: %w", err)
	}

	// TODO(forrest): register routes in a single well defined location, not scattered everywhere, makes confilicts hard
	p.Echo.POST("/", echo.WrapHandler(storage.NewHandler(ucanSvr)))

	return &Server{ucanSvr}, nil
}
