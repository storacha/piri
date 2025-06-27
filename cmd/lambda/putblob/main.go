package main

import (
	"github.com/storacha/piri/cmd/lambda"
	"github.com/storacha/piri/pkg/aws"
)

func main() {
	lambda.StartHTTPHandler(aws.ConstructBlobPutHandler)
}
