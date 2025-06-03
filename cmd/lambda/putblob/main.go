package main

import (
	"net/http"

	"github.com/storacha/piri/cmd/lambda"
	"github.com/storacha/piri/pkg/aws"
	"github.com/storacha/piri/pkg/service/blobs"
)

func main() {
	lambda.StartHTTPHandler(makeHandler)
}

func makeHandler(cfg aws.Config) (http.Handler, error) {
	service, err := aws.Construct(cfg)
	if err != nil {
		return nil, err
	}

	handler := blobs.NewBlobPutHandler(service.Blobs().Presigner(), service.Blobs().Allocations(), service.Blobs().Store())
	return handler, nil
}
