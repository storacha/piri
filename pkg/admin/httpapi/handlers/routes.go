package handlers

import (
	"crypto/ed25519"
	"fmt"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang-jwt/jwt/v4"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/admin/httpapi"
	"github.com/storacha/piri/pkg/config/app"
	echofx "github.com/storacha/piri/pkg/fx/echo"
	"github.com/storacha/piri/pkg/pdp/ethereum"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
)

type AdminRoutes struct {
	jwtMiddleware  echo.MiddlewareFunc
	paymentHandler *PaymentHandler
}

type NewRoutesParams struct {
	fx.In

	Identity         app.IdentityConfig
	Payment          smartcontracts.Payment          `optional:"true"`
	PDPConfig        app.PDPServiceConfig            `optional:"true"`
	ServiceView      smartcontracts.Service          `optional:"true"`
	ServiceValidator smartcontracts.ServiceValidator `optional:"true"`
	EthClient        *ethclient.Client               `optional:"true"`
	Sender           ethereum.Sender                 `optional:"true"`
}

func NewRoutes(params NewRoutesParams) (echofx.RouteRegistrar, error) {
	if params.Identity.Signer == nil {
		return nil, fmt.Errorf("missing identity signer for jwt auth")
	}
	publicKey := ed25519.PublicKey(params.Identity.Signer.Verifier().Raw())
	jwtMiddleware := echojwt.WithConfig(echojwt.Config{
		SigningKey:    publicKey,
		SigningMethod: jwt.SigningMethodEdDSA.Alg(),
	})

	var paymentHandler *PaymentHandler
	if params.Payment != nil {
		paymentHandler = NewPaymentHandler(params.Payment, params.PDPConfig, params.ServiceView, params.ServiceValidator, params.EthClient, params.Sender)
	}

	return &AdminRoutes{
		jwtMiddleware:  jwtMiddleware,
		paymentHandler: paymentHandler,
	}, nil
}

func (a *AdminRoutes) RegisterRoutes(e *echo.Echo) {
	adminGroup := e.Group(httpapi.AdminRoutePath, a.jwtMiddleware)

	logGroup := adminGroup.Group(httpapi.LogRoutePath)
	logGroup.GET("/list", listLogLevels)
	logGroup.POST("/set", setLogLevel)
	logGroup.POST("/set-regex", setLogLevelRegex)

	if a.paymentHandler != nil {
		paymentGroup := adminGroup.Group(httpapi.PaymentRoutePath)
		paymentGroup.GET("/account", a.paymentHandler.GetAccountInfo)
		paymentGroup.GET("/settle/:railId/estimate", a.paymentHandler.EstimateSettlement)
		paymentGroup.POST("/settle/:railId", a.paymentHandler.SettleRail)
	}
}
