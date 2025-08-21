package main

import (
	"net/http"

	ucanserver "github.com/storacha/go-ucanto/server"

	logging "github.com/ipfs/go-log/v2"

	"github.com/storacha/piri/cmd/lambda"
	"github.com/storacha/piri/internal/telemetry"
	"github.com/storacha/piri/pkg/aws"
	"github.com/storacha/piri/pkg/principalresolver"
	"github.com/storacha/piri/pkg/service/storage"
)

var log = logging.Logger("lambda/postroot")

func main() {
	lambda.StartHTTPHandler(makeHandler)
}

func makeHandler(cfg aws.Config) (http.Handler, error) {
	service, err := aws.Construct(cfg)
	if err != nil {
		return nil, err
	}

	presolv, err := principalresolver.NewMapResolver(cfg.PrincipalMapping)
	if err != nil {
		return nil, err
	}

	server, err := storage.NewUCANServer(service, ucanserver.WithPrincipalResolver(presolv.ResolveDIDKey))
	if err != nil {
		return nil, err
	}

	handler := storage.NewHandler(server)
	return telemetry.NewErrorReportingHandler(func(w http.ResponseWriter, r *http.Request) error {
		err := handler(aws.NewHandlerContext(w, r))
		if err != nil {
			log.Error(err)
		}
		return err
	}), nil
}
