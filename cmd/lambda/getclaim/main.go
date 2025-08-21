package main

import (
	"net/http"

	"github.com/storacha/piri/cmd/lambda"
	"github.com/storacha/piri/internal/telemetry"
	"github.com/storacha/piri/pkg/aws"
	"github.com/storacha/piri/pkg/service/claims"
)

func main() {
	lambda.StartHTTPHandler(makeHandler)
}

func makeHandler(cfg aws.Config) (http.Handler, error) {
	service, err := aws.Construct(cfg)
	if err != nil {
		return nil, err
	}

	handler := claims.NewHandler(service.Claims().Store())
	return telemetry.NewErrorReportingHandler(func(w http.ResponseWriter, r *http.Request) error {
		return handler(aws.NewHandlerContext(w, r))
	}), nil
}
