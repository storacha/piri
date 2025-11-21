package main

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/labstack/gommon/log"
	"github.com/storacha/go-libstoracha/ipnipublisher/publisher"
	"github.com/storacha/go-libstoracha/ipnipublisher/queue"
	awspublishingqueue "github.com/storacha/go-libstoracha/ipnipublisher/queue/aws"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-libstoracha/metadata"
	"github.com/storacha/piri/cmd/lambda"
	"github.com/storacha/piri/pkg/aws"
)

const gracePeriod = time.Second

func main() {
	lambda.StartBatchSQSEventHandler(makeHandler)
}

func makeHandler(cfg aws.Config) (lambda.SQSBatchEventHandler, error) {
	sqsPublishingDecoder := awspublishingqueue.NewSQSPublishingDecoder(cfg.Config, cfg.PublishingBucket)
	ipniStore := aws.NewS3Store(cfg.Config, cfg.IPNIStoreBucket, cfg.IPNIStorePrefix, cfg.S3Options...)
	chunkLinksTable := aws.NewDynamoProviderContextTable(cfg.Config, cfg.ChunkLinksTableName, cfg.DynamoOptions...)
	metadataTable := aws.NewDynamoProviderContextTable(cfg.Config, cfg.MetadataTableName, cfg.DynamoOptions...)
	publisherStore := store.NewPublisherStore(ipniStore, chunkLinksTable, metadataTable, store.WithMetadataContext(metadata.MetadataContext))
	advertisementPublishingQueue := awspublishingqueue.NewSQSAdvertisementPublishingQueue(cfg.Config, cfg.SQSAdvertisementPublishingQueueID)
	advertismentQueuePublisher := queue.NewAdvertisementQueuePublisher(advertisementPublishingQueue, publisherStore)

	return func(ctx context.Context, sqsEvent events.SQSEvent) (events.SQSEventResponse, error) {
		deadline, ok := ctx.Deadline()
		if ok {
			graceDeadline := deadline.Add(-gracePeriod)
			// if graceful shutdown time is after now then we can apply new deadline
			if graceDeadline.After(time.Now()) {
				dctx, cancel := context.WithDeadline(ctx, graceDeadline)
				defer cancel()
				ctx = dctx
			}
		}

		failures := make([]events.SQSBatchItemFailure, 0, len(sqsEvent.Records))
		for _, msg := range sqsEvent.Records {
			err := handleMessage(ctx, sqsPublishingDecoder, advertismentQueuePublisher, msg)
			if err != nil {
				failures = append(failures, events.SQSBatchItemFailure{
					ItemIdentifier: msg.MessageId,
				})
				log.Errorf("unable to process message %s: %s", msg.MessageId, err.Error())
			}
		}
		return events.SQSEventResponse{BatchItemFailures: failures}, nil
	}, nil
}

func handleMessage(ctx context.Context, sqsPublishingDecoder *awspublishingqueue.SQSPublishingDecoder, publisher publisher.AsyncPublisher, msg events.SQSMessage) error {
	job, err := sqsPublishingDecoder.DecodeMessage(ctx, msg.ReceiptHandle, msg.Body)
	if err != nil {
		return err
	}
	err = publisher.Publish(ctx, job.Job.ProviderInfo, job.Job.ContextID, job.Job.Digests, job.Job.Meta)
	// Do not hold up the queue by re-attempting a cache job that times out. It is
	// probably a big DAG and retrying is unlikely to subsequently succeed.
	if errors.Is(err, context.DeadlineExceeded) {
		log.Warnf("not retrying cache provider job for: %s error: %s", job.Job.ContextID, err)
		return nil
	}
	if err != nil {
		return err
	}
	return nil
}
