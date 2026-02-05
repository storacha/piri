package handlers

import (
	"crypto/ed25519"
	"fmt"

	"github.com/golang-jwt/jwt/v4"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/admin/httpapi"
	"github.com/storacha/piri/pkg/config/app"
	echofx "github.com/storacha/piri/pkg/fx/echo"
)

type AdminRoutes struct {
	jwtMiddleware  echo.MiddlewareFunc
	paymentHandler *PaymentHandler
}

type NewRoutesParams struct {
	fx.In

	Identity       app.IdentityConfig
	PaymentHandler *PaymentHandler `optional:"true"`
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

	return &AdminRoutes{
		jwtMiddleware:  jwtMiddleware,
		paymentHandler: params.PaymentHandler,
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
		paymentGroup.GET("/settle/:railId/status", a.paymentHandler.GetSettlementStatus)
		paymentGroup.POST("/settle/:railId", a.paymentHandler.SettleRail)
		paymentGroup.POST("/withdraw/estimate", a.paymentHandler.EstimateWithdraw)
		paymentGroup.POST("/withdraw", a.paymentHandler.Withdraw)
		paymentGroup.GET("/withdraw/status", a.paymentHandler.GetWithdrawalStatus)
	}
}
