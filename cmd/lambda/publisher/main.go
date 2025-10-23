package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/labstack/gommon/log"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/multiformats/go-multiaddr"
	"github.com/storacha/go-libstoracha/ipnipublisher/publisher"
	awspublisherqueue "github.com/storacha/go-libstoracha/ipnipublisher/queue/aws"
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
	sqsPublishingDecoder := awspublisherqueue.NewSQSPublishingDecoder(cfg.Config, cfg.PublishingBucket)
	ipniStore := aws.NewS3Store(cfg.Config, cfg.IPNIStoreBucket, cfg.IPNIStorePrefix, cfg.S3Options...)
	chunkLinksTable := aws.NewDynamoProviderContextTable(cfg.Config, cfg.ChunkLinksTableName, cfg.DynamoOptions...)
	metadataTable := aws.NewDynamoProviderContextTable(cfg.Config, cfg.MetadataTableName, cfg.DynamoOptions...)
	publisherStore := store.NewPublisherStore(ipniStore, chunkLinksTable, metadataTable, store.WithMetadataContext(metadata.MetadataContext))
	priv, err := crypto.UnmarshalEd25519PrivateKey(cfg.Signer.Raw())
	if err != nil {
		return nil, fmt.Errorf("unmarshaling private key: %w", err)
	}
	announceAddr, err := multiaddr.NewMultiaddr(cfg.IPNIPublisherAnnounceAddress)
	if err != nil {
		return nil, fmt.Errorf("parsing announce multiaddr: %w", err)
	}

	opts := []publisher.Option{publisher.WithAnnounceAddrs(announceAddr.String())}
	for _, url := range cfg.IPNIAnnounceURLs {
		opts = append(opts, publisher.WithDirectAnnounce(url.String()))
	}
	publisher, err := publisher.New(
		priv, publisherStore,
		opts...,
	)
	if err != nil {
		return nil, fmt.Errorf("creating IPNI publisher instance: %w", err)
	}

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
			err := handleMessage(ctx, sqsPublishingDecoder, publisher, msg)
			if err != nil {
				failures = append(failures, events.SQSBatchItemFailure{
					ItemIdentifier: msg.MessageId,
				})
			}
		}
		return events.SQSEventResponse{BatchItemFailures: failures}, nil
	}, nil
}

func handleMessage(ctx context.Context, sqsPublishingDecoder *awspublisherqueue.SQSPublishingDecoder, publisher publisher.Publisher, msg events.SQSMessage) error {
	job, err := sqsPublishingDecoder.DecodeMessage(ctx, msg.ReceiptHandle, msg.Body)
	if err != nil {
		return err
	}
	_, err = publisher.Publish(ctx, job.ProviderInfo, job.ContextID, job.Digests, job.Meta)
	// Do not hold up the queue by re-attempting a cache job that times out. It is
	// probably a big DAG and retrying is unlikely to subsequently succeed.
	if errors.Is(err, context.DeadlineExceeded) {
		log.Warnf("not retrying cache provider job for: %s error: %s", job.ContextID, err)
		return nil
	}
	if err != nil {
		return err
	}
	return nil
}
