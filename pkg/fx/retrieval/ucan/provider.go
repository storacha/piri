package ucan

import (
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
	ucancap "github.com/storacha/go-libstoracha/capabilities/ucan"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/principal"
	ucanserver "github.com/storacha/go-ucanto/server"
	ucanretrieval "github.com/storacha/go-ucanto/server/retrieval"
	"github.com/storacha/go-ucanto/ucan"

	"github.com/storacha/piri/pkg/config/app"
	echofx "github.com/storacha/piri/pkg/fx/echo"
	"github.com/storacha/piri/pkg/fx/retrieval/ucan/handlers"
	"github.com/storacha/piri/pkg/service/retrieval"
	"go.uber.org/fx"
)

var log = logging.Logger("fx/retrieval/ucan")

type Handler struct {
	ucanServer ucanserver.ServerView[ucanretrieval.Service]
}

var Module = fx.Module("retrieval/ucan/server",
	fx.Provide(
		NewHandler,
		fx.Annotate(
			AsRouteRegistrar,
			fx.ResultTags(`group:"route_registrar"`),
		),
		ProvideServerView,
	),
	handlers.Module,
)

type Params struct {
	fx.In

	ID      principal.Signer
	Config  app.AppConfig
	Options []ucanretrieval.Option `group:"ucan_retrieval_options"`
}

func NewHandler(p Params) (*Handler, error) {
	// Create a local delegation to the upload service that allows it to issue
	// attestations. When the validator sees this delegation, it will accept
	// attestations issued by the upload service.
	attestDlg, err := delegation.Delegate(
		p.ID,
		p.Config.UCANService.Services.Upload.Connection.ID(),
		[]ucan.Capability[ucan.NoCaveats]{
			ucan.NewCapability(
				ucancap.AttestAbility,
				p.ID.DID().String(),
				ucan.NoCaveats{},
			),
		},
		delegation.WithNoExpiration(),
	)
	if err != nil {
		return nil, err
	}

	options := []ucanretrieval.Option{
		ucanretrieval.WithAuthorityProofs(attestDlg),
		ucanretrieval.WithErrorHandler(func(err ucanserver.HandlerExecutionError[any]) {
			l := log.With("error", err.Error())
			if s := err.Stack(); s != "" {
				l = l.With("stack", s)
			}
			l.Error("ucan retrieval handler execution error")
		}),
	}
	options = append(options, p.Options...)
	ucanSvr, err := ucanretrieval.NewServer(p.ID, options...)
	if err != nil {
		return nil, fmt.Errorf("creating ucan retrieval server: %w", err)
	}

	return &Handler{ucanSvr}, nil
}

// RegisterRoutes registers the UCAN routes with Echo
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.GET("/piece/:cid", retrieval.NewHandler(h.ucanServer))
}

// AsRouteRegistrar provides the Handler as a RouteRegistrar
func AsRouteRegistrar(h *Handler) echofx.RouteRegistrar {
	return h
}

// ProvideServerView provides the UCAN ServerView for testing
func ProvideServerView(h *Handler) ucanserver.ServerView[ucanretrieval.Service] {
	return h.ucanServer
}
