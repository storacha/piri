package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
	ucanserver "github.com/storacha/go-ucanto/server"

	"github.com/storacha/piri/cmd/lambda"
	"github.com/storacha/piri/pkg/aws"
	"github.com/storacha/piri/pkg/principalresolver"
	"github.com/storacha/piri/pkg/services"
	"github.com/storacha/piri/pkg/services/ucan"
)

func main() {
	lambda.StartHTTPHandler(makeHandler)
}

func makeHandler(cfg aws.Config) (http.Handler, error) {
	// Create principal resolver
	presolv, err := principalresolver.New(cfg.PrincipalMapping)
	if err != nil {
		return nil, err
	}

	// Create Echo instance outside fx app
	e := echo.New()
	
	// Create a temporary fx app to construct the UCAN server with all handlers
	app := fx.New(
		// Supply the config
		fx.Supply(cfg),
		
		// Supply the Echo instance
		fx.Supply(e),
		
		// Supply the principal resolver option
		fx.Supply(ucanserver.WithPrincipalResolver(presolv.ResolveDIDKey)),
		
		// Include AWS configuration module
		aws.ConfigModule,
		
		// Include AWS datastores
		fx.Provide(
			aws.ProvideAWSBlobstore,
			aws.ProvideAWSAllocationStore,
			aws.ProvideAWSClaimStore,
			aws.ProvideAWSPublisherStore,
			aws.ProvideAWSReceiptStore,
		),
		
		// Include service implementations
		services.ServiceModule,
		
		// Include UCAN handlers
		services.UCANMethodsModule,
		
		// Include UCAN server
		ucan.Module,
		
		// Don't start the lifecycle (we just want to construct the services)
		fx.NopLogger,
	)
	
	if err := app.Err(); err != nil {
		return nil, err
	}
	
	return e, nil
}
