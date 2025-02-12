package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime/datamodel"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/storacha/go-capabilities/pkg/types"
	"github.com/storacha/storage/cmd/lambda"
	"github.com/storacha/storage/internal/ipldstore"
	"github.com/storacha/storage/pkg/aws"
	"github.com/storacha/storage/pkg/pdp/aggregator"
	"github.com/storacha/storage/pkg/pdp/aggregator/aggregate"
	"github.com/storacha/storage/pkg/store/receiptstore"
)

func main() {
	lambda.StartSQSEventHandler(makeHandler)
}

func makeHandler(cfg aws.Config) (lambda.SQSEventHandler, error) {
	aggregateStore := ipldstore.IPLDStore[datamodel.Link, aggregate.Aggregate](
		aws.NewS3Store(cfg.Config, cfg.AggregatesBucket, cfg.AggregatesPrefix),
		aggregate.AggregateType(), types.Converters...)
	ranLinkIndex := aws.NewDynamoRanLinkIndex(cfg.Config, cfg.RanLinkIndexTableName)
	s3ReceiptStore := aws.NewS3Store(cfg.Config, cfg.ReceiptStoreBucket, cfg.ReceiptStorePrefix)
	receiptStore, err := receiptstore.NewReceiptStore(s3ReceiptStore, ranLinkIndex)
	if err != nil {
		return nil, fmt.Errorf("setting up receipt store: %w", err)
	}
	pieceAccepter := aggregator.NewPieceAccepter(cfg.Signer, aggregateStore, receiptStore)

	return func(ctx context.Context, sqsEvent events.SQSEvent) error {
		// process messages in parallel
		aggregateLinks := make([]datamodel.Link, 0, len(sqsEvent.Records))
		for _, msg := range sqsEvent.Records {
			var pieceLinkMessage aws.PieceLinkMessage
			err := json.Unmarshal([]byte(msg.Body), &pieceLinkMessage)
			if err != nil {
				return fmt.Errorf("deserializing message json: %w", err)
			}
			c, err := cid.Decode(pieceLinkMessage.Piece)
			if err != nil {
				return fmt.Errorf("decoding piece link: %w", err)
			}

			aggregateLinks = append(aggregateLinks, cidlink.Link{Cid: c})
		}
		return pieceAccepter.AcceptPieces(ctx, aggregateLinks)
	}, nil
}
