package lambda

import (
	"context"
	"fmt"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"github.com/storacha/piri/internal/telemetry"
	"github.com/storacha/piri/pkg/aws"
)

// SQSEventHandler is a function that handles SQS events, suitable to use as a lambda handler.
type SQSEventHandler func(context.Context, events.SQSEvent) error

// SQSEventHandlerBuilder is a function that creates a SQSEventHandler from a config.
type SQSEventHandlerBuilder func(aws.Config) (SQSEventHandler, error)

// StartSQSEventHandler starts a lambda handler that processes SQS events.
func StartSQSEventHandler(makeHandler SQSEventHandlerBuilder) {
	ctx := context.Background()
	cfg := aws.FromEnv(ctx)
	telemetry.SetupErrorReporting(cfg.SentryDSN, cfg.SentryEnvironment)

	handler, err := makeHandler(cfg)
	if err != nil {
		telemetry.ReportError(ctx, err)
		panic(err)
	}

	lambda.StartWithOptions(instrumentSQSEventHandler(handler), lambda.WithContext(ctx))
}

// instrumentSQSEventHandler wraps a SQSEventHandler with error reporting.
func instrumentSQSEventHandler(handler SQSEventHandler) SQSEventHandler {
	return func(ctx context.Context, sqsEvent events.SQSEvent) error {
		err := handler(ctx, sqsEvent)
		if err != nil {
			telemetry.ReportError(ctx, err)
		}

		return err
	}
}

// SQSBatchEventHandler is a function that handles SQS events, suitable to use as a lambda handler.
type SQSBatchEventHandler func(context.Context, events.SQSEvent) (events.SQSEventResponse, error)

// SQSBatchEventHandlerBuilder is a function that creates a SQSBatchEventHandler from a config.
type SQSBatchEventHandlerBuilder func(aws.Config) (SQSBatchEventHandler, error)

// StartBatchSQSEventHandler starts a lambda handler that processes SQS events.
func StartBatchSQSEventHandler(makeHandler SQSBatchEventHandlerBuilder) {
	ctx := context.Background()
	cfg := aws.FromEnv(ctx)
	telemetry.SetupErrorReporting(cfg.SentryDSN, cfg.SentryEnvironment)

	handler, err := makeHandler(cfg)
	if err != nil {
		telemetry.ReportError(ctx, err)
		panic(err)
	}

	lambda.StartWithOptions(instrumentSQSBatchEventHandler(handler), lambda.WithContext(ctx))
}

// instrumentSQSBatchEventHandler wraps a SQSBatchEventHandler with error reporting.
func instrumentSQSBatchEventHandler(handler SQSBatchEventHandler) SQSBatchEventHandler {
	return func(ctx context.Context, sqsEvent events.SQSEvent) []events.SQSBatchItemFailure {
		failures := handler(ctx, sqsEvent)
		if len(failures) > 0 {
			telemetry.ReportError(ctx, fmt.Errorf("handling batch SQS event failed: %v", failures))
		}
		return failures
	}
}

// HTTPHandlerBuilder is a function that creates a http.Handler from a config.
type HTTPHandlerBuilder func(aws.Config) (http.Handler, error)

// StartHTTPHandler starts a lambda handler that processes HTTP requests.
func StartHTTPHandler(makeHandler HTTPHandlerBuilder) {
	ctx := context.Background()
	cfg := aws.FromEnv(ctx)
	telemetry.SetupErrorReporting(cfg.SentryDSN, cfg.SentryEnvironment)

	handler, err := makeHandler(cfg)
	if err != nil {
		telemetry.ReportError(ctx, err)
		panic(err)
	}

	lambda.StartWithOptions(httpadapter.NewV2(handler).ProxyWithContext, lambda.WithContext(ctx))
}
