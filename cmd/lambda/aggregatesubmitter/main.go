package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/aws/aws-lambda-go/events"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime/datamodel"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/storacha/go-libstoracha/capabilities/types"

	"github.com/storacha/piri/cmd/lambda"
	"github.com/storacha/piri/internal/ipldstore"
	"github.com/storacha/piri/pkg/aws"
	"github.com/storacha/piri/pkg/pdp/aggregator"
	"github.com/storacha/piri/pkg/pdp/aggregator/aggregate"
	"github.com/storacha/piri/pkg/pdp/httpapi/client"
)

func main() {
	lambda.StartSQSEventHandler(makeHandler)
}

func makeHandler(cfg aws.Config) (lambda.SQSEventHandler, error) {
	pdpServerURL, err := url.Parse(cfg.PDPServerURL)
	if err != nil {
		return nil, fmt.Errorf("parsing pdp server URL: %w", err)
	}

	apiClient, err := client.New(pdpServerURL, client.WithBearerFromSigner(cfg.Signer))
	if err != nil {
		return nil, fmt.Errorf("creating api client: %w", err)
	}
	aggregateStore := ipldstore.IPLDStore[datamodel.Link, aggregate.Aggregate](
		aws.NewS3Store(cfg.Config, cfg.AggregatesBucket, cfg.AggregatesPrefix),
		aggregate.AggregateType(), types.Converters...)
	aggregateSubmitterQueue := aws.NewSQSAggregateQueue(cfg.Config, cfg.SQSPDPPieceAggregatorURL)
	aggregateSubmitter := aggregator.NewAggregateSubmitteer(
		&aggregator.ConfiguredProofSetProvider{ID: cfg.PDPProofSet},
		aggregateStore,
		apiClient,
		aggregateSubmitterQueue,
	)

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
		return aggregateSubmitter.SubmitAggregates(ctx, aggregateLinks)
	}, nil
}
