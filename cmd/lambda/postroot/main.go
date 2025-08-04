package main

import (
	"net/http"

	ucanserver "github.com/storacha/go-ucanto/server"

	"github.com/storacha/piri/cmd/lambda"
	"github.com/storacha/piri/pkg/aws"
	"github.com/storacha/piri/pkg/principalresolver"
	"github.com/storacha/piri/pkg/service/storage"
)

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

	return storage.NewHandler(server), nil
}
