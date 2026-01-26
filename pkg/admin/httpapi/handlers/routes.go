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
	"github.com/storacha/piri/pkg/config/dynamic"
	echofx "github.com/storacha/piri/pkg/fx/echo"
)

type AdminRoutes struct {
	jwtMiddleware  echo.MiddlewareFunc
	paymentHandler *PaymentHandler
	configHandler  *ConfigHandler
}

type AdminRoutesParams struct {
	fx.In

	Identity       app.IdentityConfig
	PaymentHandler *PaymentHandler `optional:"true"`
	Registry       *dynamic.Registry
	Bridge         *dynamic.ViperBridge
}

func NewRoutes(params AdminRoutesParams) (echofx.RouteRegistrar, error) {
	if params.Identity.Signer == nil {
		return nil, fmt.Errorf("missing identity signer for jwt auth")
	}
	publicKey := ed25519.PublicKey(params.Identity.Signer.Verifier().Raw())
	jwtMiddleware := echojwt.WithConfig(echojwt.Config{
		SigningKey:    publicKey,
		SigningMethod: jwt.SigningMethodEdDSA.Alg(),
	})

	var configHandler *ConfigHandler
	if params.Registry != nil {
		configHandler = NewConfigHandler(params.Registry, params.Bridge)

	}
	return &AdminRoutes{
		jwtMiddleware:  jwtMiddleware,
		paymentHandler: params.PaymentHandler,
		configHandler:  configHandler,
	}, nil
}

func (a *AdminRoutes) RegisterRoutes(e *echo.Echo) {
	adminGroup := e.Group(httpapi.AdminRoutePath, a.jwtMiddleware)

	// Log routes
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

	// Config routes (only if dynamic config is enabled)
	if a.configHandler != nil {
		configGroup := adminGroup.Group(httpapi.ConfigRoutePath)
		configGroup.GET("", a.configHandler.GetConfig)
		configGroup.PATCH("", a.configHandler.UpdateConfig)
		configGroup.POST(httpapi.ConfigReloadRoutePath, a.configHandler.ReloadConfig)
	}
}
