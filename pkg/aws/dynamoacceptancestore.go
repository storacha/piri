package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	multihash "github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/digestutil"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/acceptancestore"
	"github.com/storacha/piri/pkg/store/acceptancestore/acceptance"
)

// DynamoAcceptanceStore implements the AcceptanceStore interface on dynamodb
type DynamoAcceptanceStore struct {
	tableName      string
	dynamoDbClient *dynamodb.Client
}

// NewDynamoAcceptanceStore returns an AcceptanceStore connected to a AWS DynamoDB table
func NewDynamoAcceptanceStore(cfg aws.Config, tableName string, opts ...func(*dynamodb.Options)) *DynamoAcceptanceStore {
	return &DynamoAcceptanceStore{
		tableName:      tableName,
		dynamoDbClient: dynamodb.NewFromConfig(cfg, opts...),
	}
}

func (d *DynamoAcceptanceStore) Get(ctx context.Context, mh multihash.Multihash, space did.DID) (acceptance.Acceptance, error) {
	res, err := d.dynamoDbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(d.tableName),
		Key: map[string]types.AttributeValue{
			"hash":  &types.AttributeValueMemberS{Value: digestutil.Format(mh)},
			"space": &types.AttributeValueMemberS{Value: space.String()},
		},
	})
	if err != nil {
		return acceptance.Acceptance{}, fmt.Errorf("getting item: %w", err)
	}

	if res.Item == nil {
		return acceptance.Acceptance{}, store.ErrNotFound
	}
	var item acceptanceItem
	err = attributevalue.UnmarshalMap(res.Item, &item)
	if err != nil {
		return acceptance.Acceptance{}, fmt.Errorf("unmarshalling acceptance item: %w", err)
	}
	acc, err := acceptance.Decode(item.Acceptance, dagcbor.Decode)
	if err != nil {
		return acceptance.Acceptance{}, fmt.Errorf("decoding acceptance: %w", err)
	}
	return acc, nil
}

// List implements acceptancestore.AcceptanceStore.
func (d *DynamoAcceptanceStore) List(ctx context.Context, mh multihash.Multihash, options ...acceptancestore.ListOption) ([]acceptance.Acceptance, error) {
	cfg := acceptancestore.ListConfig{}
	for _, opt := range options {
		opt(&cfg)
	}

	keyEx := expression.Key("hash").Equal(expression.Value(digestutil.Format(mh)))
	expr, err := expression.NewBuilder().WithKeyCondition(keyEx).Build()
	if err != nil {
		return nil, fmt.Errorf("building query: %w", err)
	}

	var limit *int32
	if cfg.Limit > 0 {
		limit = aws.Int32(int32(cfg.Limit))
	}

	var acceptances []acceptance.Acceptance
	queryPaginator := dynamodb.NewQueryPaginator(d.dynamoDbClient, &dynamodb.QueryInput{
		TableName:                 aws.String(d.tableName),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ConsistentRead:            aws.Bool(true),
		Limit:                     limit,
	})
	for queryPaginator.HasMorePages() {
		response, err := queryPaginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("querying acceptances: %w", err)
		}
		var acceptancePage []acceptanceItem
		err = attributevalue.UnmarshalListOfMaps(response.Items, &acceptancePage)
		if err != nil {
			return nil, fmt.Errorf("parsing query responses: %w", err)
		}

		for _, item := range acceptancePage {
			a, err := acceptance.Decode(item.Acceptance, dagcbor.Decode)
			if err != nil {
				return nil, fmt.Errorf("decoding data: %w", err)
			}
			acceptances = append(acceptances, a)
		}
	}
	return acceptances, nil
}

// Put implements acceptancestore.AcceptanceStore.
func (d *DynamoAcceptanceStore) Put(ctx context.Context, acc acceptance.Acceptance) error {
	data, err := acceptance.Encode(acc, dagcbor.Encode)
	if err != nil {
		return fmt.Errorf("encoding data: %w", err)
	}
	item, err := attributevalue.MarshalMap(acceptanceItem{
		Hash:       digestutil.Format(acc.Blob.Digest),
		Space:      acc.Space.String(),
		Acceptance: data,
	})
	if err != nil {
		return fmt.Errorf("serializing item: %w", err)
	}
	_, err = d.dynamoDbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.tableName), Item: item,
	})
	if err != nil {
		return fmt.Errorf("storing item: %w", err)
	}
	return nil
}

type acceptanceItem struct {
	Hash       string `dynamodbav:"hash"`
	Space      string `dynamodbav:"space"`
	Acceptance []byte `dynamodbav:"acceptance"`
}

var _ acceptancestore.AcceptanceStore = (*DynamoAcceptanceStore)(nil)
