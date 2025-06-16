package main

import (
	"fmt"
	"net/http"

	"github.com/storacha/piri/cmd/lambda"
	"github.com/storacha/piri/pkg/aws"
	"github.com/storacha/piri/pkg/build"
)

func main() {
	lambda.StartHTTPHandler(makeHandler)
}

func makeHandler(cfg aws.Config) (http.Handler, error) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf("🔥 piri %s\n", build.Version)))
		w.Write([]byte("- https://github.com/storacha/piri\n"))
		w.Write([]byte(fmt.Sprintf("- %s", cfg.Signer)))
	}), nil
}
